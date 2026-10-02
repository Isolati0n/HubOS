# Deliberately broken inventory files

Each file breaks exactly one rule from `docs/inventory-format.md` and is otherwise valid. The exact message `hubd` must print is in the `# EXPECT:` line at the top of the file; `go test ./internal/inventory` fails if the real output differs. Files 22 to 24 test rules taken from the field table (allowed values, no port in the address, port range). File 25 checks that a decimal format number is described as a decimal number. File 26 tests the control-character rule.

| File | Rule it breaks |
|---|---|
| `01-format-missing.toml` | the format line is missing |
| `02-format-unsupported.toml` | format value is not a supported version |
| `03-id-duplicate.toml` | id must be unique |
| `04-id-bad-characters.toml` | id uses only lowercase letters, digits and dashes |
| `05-open-empty.toml` | open must not be empty |
| `06-open-none-with-other.toml` | none only as the sole entry in open |
| `07-share-without-files.toml` | share only when open includes files |
| `08-guest-missing-host.toml` | a guest must have host |
| `09-guest-missing-lifetime.toml` | a guest must have lifetime |
| `10-host-on-non-guest.toml` | host is refused on every role except guest |
| `11-lifetime-on-non-guest.toml` | lifetime is refused on every role except guest |
| `12-host-not-vm-host.toml` | host must point at a vm-host |
| `13-host-does-not-exist.toml` | host must point at a machine that exists |
| `14-home-duplicate.toml` | two machines may not share the same home |
| `15-two-hubs.toml` | exactly one hub: more than one |
| `16-no-hub.toml` | exactly one hub: none |
| `17-unknown-field.toml` | an unknown field is an error |
| `18-role-invalid.toml` | role must be one of the eight |
| `19-open-value-invalid.toml` | open entries must be known programs |
| `20-required-field-missing.toml` | a required field is absent |
| `21-not-toml.toml` | the file is not valid TOML |
| `22-lifetime-invalid.toml` | lifetime must be ephemeral or persistent |
| `23-address-has-port.toml` | address has no port in it |
| `24-port-out-of-range.toml` | port must be 1 to 65535 |
| `25-format-decimal.toml` | a decimal format number is reported as a decimal number |
| `26-control-character.toml` | id, name, user, share and address have no control characters or line breaks |
