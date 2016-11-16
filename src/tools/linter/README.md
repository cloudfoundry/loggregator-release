# Linter

The linter searches the codebase for code that has been identified as bad patterns.

## Usage
```
    go run cmd/linter/main.go [--locks-only=<true|false>]
--path=<path/subpath|path/...>...
```
The output contains the pattern matched, the file/line number/column and a
snippet of the code surrounding the matched pattern.

The flags that can be passed to the linter are:

| Flag               | Required                    | Description                                                                                         |
|--------------------|-----------------------------|-----------------------------------------------------------------------------------------------------|
| ```--path```       | Yes                         | The directory to search in, relative to the gopath. Multiple allowed. In form '___/___/ or ___/...' |
| ```--locks-only``` | Yes                         | Only output matched patterns that include locks                                                     |
