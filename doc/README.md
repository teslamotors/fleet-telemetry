# License management
install license finder https://github.com/pivotal/LicenseFinder
(all commands from project root)

## List unapproved licenses
`license_finder --decisions-file=doc/license_decisions.yml`

## Add/Remove approvals
ex.: `license_finder permitted_licenses add "Apache 2.0" --decisions-file=doc/license_decisions.yml`

## Update all_licenses.txt:
`license_finder > doc/all_licenses.txt`
