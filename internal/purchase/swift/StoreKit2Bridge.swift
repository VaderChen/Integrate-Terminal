import Foundation
import StoreKit

private let baseProductID = "pro_unlock"

private struct BridgePayload: Codable {
    let productId: String
    let planName: String
    let source: String
    let statusMessage: String
    let proUnlock: Bool
    let canPurchase: Bool
    let canRestore: Bool
    let error: String?
}

private enum BridgeError: LocalizedError {
    case productMissing
    case pendingApproval
    case unverifiedTransaction
    case restoreNotFound

    var errorDescription: String? {
        switch self {
        case .productMissing:
            return "找不到 Pro 商品，請確認 App Store Connect 商品代碼是否為 com.vader.integterm.pro_unlock 或 pro_unlock"
        case .pendingApproval:
            return "購買已延後，等待 Apple 核准"
        case .unverifiedTransaction:
            return "交易驗證失敗，無法確認授權狀態"
        case .restoreNotFound:
            return "未找到可還原的 Pro 購買紀錄"
        }
    }
}

private func makePayload(unlocked: Bool, source: String, message: String? = nil, error: String? = nil) -> BridgePayload {
    BridgePayload(
        productId: preferredProductID(),
        planName: unlocked ? "Pro" : "Free",
        source: source,
        statusMessage: message ?? (unlocked ? "目前已解鎖 Pro，連線數量無限制" : "目前為 Free，最多可開啟 2 個連線"),
        proUnlock: unlocked,
        canPurchase: !unlocked,
        canRestore: true,
        error: error
    )
}

private func encodePayload(_ payload: BridgePayload) -> UnsafeMutablePointer<CChar>? {
    let encoder = JSONEncoder()
    guard let data = try? encoder.encode(payload),
          let json = String(data: data, encoding: .utf8) else {
        let fallback = #"{"productId":"pro_unlock","planName":"Free","source":"storekit2","statusMessage":"序列化授權資料失敗","proUnlock":false,"canPurchase":true,"canRestore":true,"error":"serialize payload failed"}"#
        return strdup(fallback)
    }
    return strdup(json)
}

private func preferredProductID() -> String {
    if let bundleID = Bundle.main.bundleIdentifier?.trimmingCharacters(in: .whitespacesAndNewlines),
       !bundleID.isEmpty {
        return "\(bundleID).\(baseProductID)"
    }
    return baseProductID
}

private func candidateProductIDs() -> [String] {
    var seen = Set<String>()
    var ids: [String] = []

    for id in [preferredProductID(), baseProductID] {
        let normalized = id.trimmingCharacters(in: .whitespacesAndNewlines)
        if normalized.isEmpty || seen.contains(normalized) {
            continue
        }
        seen.insert(normalized)
        ids.append(normalized)
    }

    return ids
}

private func loadProProduct() async throws -> Product {
    let ids = candidateProductIDs()
    let products = try await Product.products(for: ids)

    for id in ids {
        if let product = products.first(where: { $0.id == id }) {
            return product
        }
    }

    throw BridgeError.productMissing
}

private func bridgeCall(timeout: DispatchTimeInterval = .seconds(600), _ work: @escaping () async -> BridgePayload) -> UnsafeMutablePointer<CChar>? {
    let semaphore = DispatchSemaphore(value: 0)
    let timeoutPayload = makePayload(unlocked: false, source: "storekit2", message: "等待 StoreKit 2 回應逾時", error: "等待 StoreKit 2 回應逾時")
    var payload = timeoutPayload

    Task {
        payload = await work()
        semaphore.signal()
    }

    if semaphore.wait(timeout: .now() + timeout) == .timedOut {
        // The task may still write payload after the wait expires. Only a
        // successful wait synchronizes that write with the read below.
        return encodePayload(timeoutPayload)
    }
    return encodePayload(payload)
}

private func purchasePayload() async -> BridgePayload {
    do {
        let product = try await loadProProduct()

        let result = try await product.purchase()
        switch result {
        case .success(let verification):
            switch verification {
            case .verified(let transaction):
                await transaction.finish()
                return BridgePayload(
                    productId: transaction.productID,
                    planName: "Pro",
                    source: "storekit2-transaction",
                    statusMessage: "Pro 購買完成，已解除連線數量限制",
                    proUnlock: true,
                    canPurchase: false,
                    canRestore: true,
                    error: nil
                )
            case .unverified:
                throw BridgeError.unverifiedTransaction
            }
        case .pending:
            throw BridgeError.pendingApproval
        case .userCancelled:
            return makePayload(unlocked: await isUnlocked(), source: "storekit2-transaction", message: "已取消購買", error: "已取消購買")
        @unknown default:
            return makePayload(unlocked: await isUnlocked(), source: "storekit2-transaction", message: "發生未知購買結果", error: "發生未知購買結果")
        }
    } catch {
        let unlocked = await isUnlocked()
        return makePayload(unlocked: unlocked, source: "storekit2-transaction", message: error.localizedDescription, error: error.localizedDescription)
    }
}

private func restorePayload() async -> BridgePayload {
    do {
        try await AppStore.sync()
        let unlocked = await isUnlocked()
        if !unlocked {
            throw BridgeError.restoreNotFound
        }
        return makePayload(unlocked: true, source: "storekit2-sync", message: "已還原 Pro 購買，連線數量限制已解除")
    } catch {
        let unlocked = await isUnlocked()
        return makePayload(unlocked: unlocked, source: "storekit2-sync", message: error.localizedDescription, error: error.localizedDescription)
    }
}

private func currentStatePayload(source: String) async -> BridgePayload {
    let unlocked = await isUnlocked()
    return makePayload(unlocked: unlocked, source: source)
}

private func isUnlocked() async -> Bool {
    let validIDs = Set(candidateProductIDs())
    for await entitlement in Transaction.currentEntitlements {
        switch entitlement {
        case .verified(let transaction):
            if validIDs.contains(transaction.productID) {
                return true
            }
        case .unverified:
            continue
        }
    }
    return false
}

@_cdecl("PurchaseBridgeCurrentStateJSON")
public func PurchaseBridgeCurrentStateJSON() -> UnsafeMutablePointer<CChar>? {
    bridgeCall {
        await currentStatePayload(source: "storekit2-entitlement")
    }
}

@_cdecl("PurchaseBridgeRefreshStateJSON")
public func PurchaseBridgeRefreshStateJSON() -> UnsafeMutablePointer<CChar>? {
    bridgeCall {
        await currentStatePayload(source: "storekit2-refresh")
    }
}

@_cdecl("PurchaseBridgePurchaseProUnlockJSON")
public func PurchaseBridgePurchaseProUnlockJSON() -> UnsafeMutablePointer<CChar>? {
    bridgeCall {
        await purchasePayload()
    }
}

@_cdecl("PurchaseBridgeRestorePurchasesJSON")
public func PurchaseBridgeRestorePurchasesJSON() -> UnsafeMutablePointer<CChar>? {
    bridgeCall {
        await restorePayload()
    }
}
