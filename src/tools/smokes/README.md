Required environment variables

- `CF_SYSTEM_DOMAIN`
- `CF_USERNAME`
- `CF_PASSWORD`
- `CF_SPACE`
- `CF_ORG`
- `NUM_APPS`
- `CYCLES`
- `DELAY_US`
- `DRAIN_URLS` a string with URLs separated by spaces
- `DATADOG_API_KEY`

Usage:

```
./push.sh && ./hammer.sh | ./report.sh && ./teardown.sh
```

*Note* There is a `|` and not `&&` between `./hammer.sh` and `./report.sh`
because `./hammer.sh` uses stdout for reporting while `./report.sh` uses
stdin.
