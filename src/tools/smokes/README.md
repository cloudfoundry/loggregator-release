Required environment variables

- `CF_SYSTEM_DOMAIN`
- `CF_USERNAME`
- `CF_PASSWORD`
- `CF_SPACE`
- `CF_ORG`
- `CYCLES`
- `DELAY_US`
- `DRAIN_URLS` a string with URLs separated by spaces
- `DATADOG_API_KEY`
- `DRAIN_VERSION`
- `NUM_APPS`

Usage:

```
./push.sh && ./hammer.sh && ./report.sh && ./teardown.sh
```
