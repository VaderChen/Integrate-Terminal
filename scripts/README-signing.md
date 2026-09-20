# App Store signing

Copy `signing.example.json` to `cert/signing.json` and keep local files inside the ignored `cert/` directory. Restrict the local configuration to the current user with `chmod 600 cert/signing.json`.

`package-appstore.sh` reads this JSON by default. `SIGNING_CONFIG` selects another file. The environment takes precedence for `TEAM_ID`, `APP_CERT`, `INSTALLER_CERT`, `BUNDLE_ID`, `PROVISION_PROFILE_PATH`, `SIGNING_KEYCHAIN`, and `CERT_DIR`.

Supply a macOS distribution provisioning profile using `PROVISION_PROFILE_PATH`, or place it at `cert/IntegTerm.provisionprofile`. The script can also find a matching profile in the current user's provisioning profile directory. The profile supplies the signing team and application identifier prefix. An optional `TEAM_ID` must match it. The script generates concrete application and Keychain identifiers in temporary entitlements and checks the profile's authorized groups.

Use `APP_CERT` and `INSTALLER_CERT` to select a certificate's full identity or SHA-1 fingerprint. If omitted, a unique matching identity is selected. The application identity must be included in the profile's `DeveloperCertificates`; installer and application identities must belong to the same team. Signing commands receive an explicit Keychain path.

Existing identities are read from the login Keychain, or from `SIGNING_KEYCHAIN`. Local `distribution.key`, `distribution.cer`, `installer.cer`, `distribution.p12`, and `installer.p12` files can be supplied under `CERT_DIR` (default `cert/`). They are imported only into a temporary signing Keychain, which is deleted on exit. Set `CERT_P12_PASSWORD` in the environment for password-protected PKCS#12 files. The script does not change the default Keychain or replace the user's Keychain search list.

Run `./package-appstore.sh` after building a production app. The source app is copied into staging before signing. The resulting package is written to `build/bin/IntegTERM.pkg`; no App Store upload is performed.
