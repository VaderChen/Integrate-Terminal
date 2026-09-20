# Native credential storage

`New()` returns a side-effect-free `Backend`. Each credential is an opaque byte
slice identified by a caller-generated, persistent random reference. The fixed
Keychain service is `com.vader.integterm.credentials.v1`; the account is the full
reference, including its schema/store namespace. Neither reference resolution
nor the service depends on an installation or data-directory path.

On macOS with cgo, the backend calls Security.framework's `SecItem` API directly.
It always selects the file-based Keychain (`kSecUseDataProtectionKeychain=false`,
`kSecAttrSynchronizable=false`) so local ad-hoc builds and sandbox builds use the
same vault. It preserves Apple's default creator-app ACL, specifies no access
group or all-app permission, and does not modify the default Keychain or search
list. OS authorization prompts remain available; denial, missing items and
locked/interaction-required conditions are distinct errors. A changed signing
identity may require Keychain authorization again. Non-macOS and no-cgo builds
return `ErrUnsupported`, with no file or plaintext fallback.

`NewNonInteractive()` selects the same vault and item ACLs for the standalone
MCP stdio process. Every native query includes
`kSecUseAuthenticationUI=kSecUseAuthenticationUIFail`. That query option alone
is not a sufficient guarantee for the file-based Keychain: Apple's current
`SecItem.h` explicitly warns that the older no-UI query option applies only to
the Data Protection Keychain, and the open-source legacy `SecItem.cpp` has no
handling for `kSecUseAuthenticationUI`.

For that reason, a noninteractive call also reads the current process's
`SecKeychainGetUserInteractionAllowed` flag, temporarily sets it to false,
performs the operation, and restores the original value in a defer. A shared
mutex covers **all native calls in this package**, including interactive
backends, so none can enter another call's suppressed-UI scope. Failure to read
or suppress the flag prevents the operation. A restore failure is returned as
an error; a successful secret is not exposed after such a failure. The default
`New()` backend never changes the flag or supplies the fail-UI query option.

This guard changes only the current process's interaction flag. It does not
unlock Keychain, approve an application, change an ACL, change the search list,
switch vaults, or alter system authorization policy. It deliberately returns
`ErrLocked` for interaction-required status and `ErrDenied` for denied access,
rather than silently skipping a protected item or treating it as missing. A
changed ad-hoc signature can therefore require the user to authorize the GUI
before its separately launched MCP process can read the saved credential.

The legacy flag APIs are deprecated but are needed for the existing file-based
vault. The guard cannot coordinate unrelated Keychain calls made outside this
package, which is why the app uses this backend only in the independent stdio
process. Suppressing authentication UI is not a general timeout for a hung
Security.framework/securityd IPC call; callers still need their own bounded
storage-operation policy where required.

Primary references for this behavior are Apple's
[UI-failure query documentation](https://developer.apple.com/documentation/security/ksecuseauthenticationuifail),
the installed Apple SDK's `Security.framework/Headers/SecItem.h` comments for
`kSecUseNoAuthenticationUI`, and Apple's open-source
[legacy SecItem implementation](https://github.com/apple-oss-distributions/Security/blob/main/OSX/libsecurity_keychain/lib/SecItem.cpp)
and [process interaction flag implementation](https://github.com/apple-oss-distributions/Security/blob/main/OSX/libsecurity_keychain/lib/SecKeychain.cpp).
The setter stores a Boolean member in
[Globals.h](https://github.com/apple-oss-distributions/Security/blob/main/OSX/libsecurity_keychain/lib/Globals.h);
[StorageManager.cpp](https://github.com/apple-oss-distributions/Security/blob/main/OSX/libsecurity_keychain/lib/StorageManager.cpp)
checks that flag before attempting authentication UI and returns
`errSecInteractionNotAllowed` when UI is disabled.

Apple documents the implementation distinction and default-vault behavior in
[TN3137](https://developer.apple.com/documentation/technotes/tn3137-on-mac-keychains),
and creator-app ACL behavior in its
[sandbox/CLI Keychain guidance](https://developer.apple.com/forums/thread/836816).
Keeping one implementation is deliberate: selecting a different vault according
to signing entitlements would make existing references stop resolving.

Normal unit tests use a fake native driver and do not access Keychain:

```sh
go test -race ./internal/credentials
CGO_ENABLED=0 go test ./internal/credentials
```

The fake-driver tests verify all three noninteractive operations, restoration
after success and failure, already-disabled UI, failure to read/set/restore the
flag, secret suppression on restore failure, and exclusion of concurrent
interactive calls. They do not prompt, alter real Keychain state, or simulate
an ACL grant.

The optional native integration test creates and removes one UUID-scoped fake
credential under the fixed service. It verifies binary data, update, lookup from
a separate backend instance, deletion and not-found behavior. It does not read
existing user credentials or change Keychain configuration:

```sh
INTEGTERM_KEYCHAIN_INTEGRATION=1 go test -race ./internal/credentials -run '^TestKeychainIntegration$' -count=1 -timeout=30s
```

The integration test's cleanup removes only its own exact service/account pair.
An interrupted process may leave that synthetic test item behind; tests never
perform a service-wide cleanup.
