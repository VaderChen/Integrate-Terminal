// Concatenate with StoreKit2Bridge.swift when compiling this harness so the
// private bridge helper can be tested without exposing another production API.
// Every work closure below is a mock; no StoreKit operation is invoked.

private func decodeBridgeResult(_ pointer: UnsafeMutablePointer<CChar>?) -> BridgePayload {
    guard let pointer else { fatalError("bridge returned a null result") }
    defer { free(pointer) }
    let data = Data(String(cString: pointer).utf8)
    return try! JSONDecoder().decode(BridgePayload.self, from: data)
}

@main
private struct StoreKit2BridgeTests {
    static func main() {
        runBridgeTests()
    }
}

// Keep mock work outside main-actor isolation, matching the C bridge callers.
private func runBridgeTests() {
        let success = decodeBridgeResult(bridgeCall(timeout: .seconds(2)) {
            makePayload(unlocked: true, source: "mock-success")
        })
        precondition(success.proUnlock && success.source == "mock-success" && success.error == nil)

        let lateCompletion = DispatchSemaphore(value: 0)
        let timeout = decodeBridgeResult(bridgeCall(timeout: .milliseconds(1)) {
            try? await Task.sleep(nanoseconds: 30_000_000)
            defer { lateCompletion.signal() }
            return makePayload(unlocked: true, source: "mock-late-success")
        })
        precondition(!timeout.proUnlock && timeout.source == "storekit2" && timeout.error != nil)
        precondition(lateCompletion.wait(timeout: .now() + .seconds(2)) == .success)
        // Give the completed work's result time to reach the bridge's task.
        Thread.sleep(forTimeInterval: 0.01)
        precondition(!timeout.proUnlock && timeout.source == "storekit2")

        // Exercise completion near the timeout boundary under ThreadSanitizer.
        // Either result is valid, but there must never be an unsynchronized read
        // of the task's mutable result or a mixture of the two payloads.
        let pendingWork = DispatchGroup()
        for _ in 0..<500 {
            pendingWork.enter()
            let result = decodeBridgeResult(bridgeCall(timeout: .microseconds(50)) {
                try? await Task.sleep(nanoseconds: 50_000)
                defer { pendingWork.leave() }
                return makePayload(unlocked: true, source: "mock-boundary")
            })
            if result.proUnlock {
                precondition(result.source == "mock-boundary" && result.error == nil)
            } else {
                precondition(result.source == "storekit2" && result.error != nil)
            }
        }
        precondition(pendingWork.wait(timeout: .now() + .seconds(2)) == .success)
        Thread.sleep(forTimeInterval: 0.01)
        print("StoreKit bridge mock tests passed (normal, timeout, late completion, boundary stress)")
}
