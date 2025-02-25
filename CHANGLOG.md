<a name="EdgeX S7 Device Service (found in device-s7) Changelog"></a>
## EdgeX S7 Device Service
[Github repository](https://github.com/edgexfoundry/device-s7)

### Change Logs for EdgeX Dependencies
- [go-mod-bootstrap](https://github.com/edgexfoundry/go-mod-bootstrap/blob/main/CHANGELOG.md)
- [go-mod-core-contracts](https://github.com/edgexfoundry/go-mod-core-contracts/blob/main/CHANGELOG.md)
- [go-mod-messaging](https://github.com/edgexfoundry/go-mod-messaging/blob/main/CHANGELOG.md)
- [go-mod-registry](https://github.com/edgexfoundry/go-mod-registry/blob/main/CHANGELOG.md) 
- [go-mod-secrets](https://github.com/edgexfoundry/go-mod-secrets/blob/main/CHANGELOG.md) (indirect dependency)
- [go-mod-configuration](https://github.com/edgexfoundry/go-mod-configuration/blob/main/CHANGELOG.md) (indirect dependency)

## [4.0.0] Odessa - 2025-03-12 (Only compatible with the 4.x releases)
### ✨  Features

- Upgrade go modules to the latest V4 versions ([e01f1b2…](https://github.com/edgexfoundry/device-s7/commit/e01f1b23293b09a895d4cc1df7e2ed2c811e9ccd))
- Optimize invalid address handling and update code to use the s7_errors variable ([f5a0aa0…](https://github.com/edgexfoundry/device-s7/commit/f5a0aa0cc3f0f12ba5a7b7b72ccceb6f71accabe))


### ♻ Code Refactoring

- Update module to v4 ([bec349e…](https://github.com/edgexfoundry/device-s7/commit/bec349e2f9af90d5dee38f24426ea0dece7bc46d))
```text

BREAKING CHANGE: update go module to v4

```

### 👷 Build

- Upgrade to go-1.23, Linter1.61.0 and Alpine 3.20 ([f7f97db…](https://github.com/edgexfoundry/device-s7/commit/f7f97db7578a3a57d460f2ed62b0c2f6f5792442))


