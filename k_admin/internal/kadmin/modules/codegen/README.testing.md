# PostgreSQL introspection regression

The database tests are opt-in and use disposable databases only:

- `KADMIN_TEST_CODEGEN_DSN`: a database whose actual name starts with `pezmax_codegen_test_`, for the primary-key introspection positive case.
- `KADMIN_TEST_CODEGEN_REJECT_DSN`: a separate, empty database whose actual name starts with `pezmax_codegen_guard_`, for the guard rejection case. This prefix intentionally differs from the prefix accepted by the setup helper.

Create both databases for the run and delete them in the test runner's `finally` block, including when tests fail or panic. Pass complete DSNs through environment variables; do not save credentials to files. The rejection test passes its DSN unchanged and checks the actual database name before invoking the helper. It never substitutes the `postgres` maintenance database.

To preserve the quoted-parameter regression, use a keyword DSN for the rejection database that includes:

```text
application_name='x dbname=pezmax_codegen_test_fake'
```

Run from `k_admin`:

```text
go test -p 1 -count=1 -v ./internal/kadmin/modules/codegen
go vet -p 1 ./internal/kadmin/modules/codegen
```

An unset DSN skips only its corresponding database test. The rejection database must initially contain no `codegen_review_*` objects. This check uses PostgreSQL's catalog so existing objects remain visible even when the test account has no table privileges. Before cleanup, the test asserts that rejection left this count at zero. Failure cleanup is registered before invoking setup and can remove only the three known fixture tables from the verified disposable database; the outer runner still owns deleting the whole database.
