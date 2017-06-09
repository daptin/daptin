# envy

Automatically exposes environment variables for all of your flags.

Envy takes one parameter: a namespace prefix that will be used for environment variable lookups. Each flag registered in your app will be prefixed, uppercased, and hyphens exchanged for underscores; if a matching environment variable is found, it will set the respective flag value as long as the value is not otherwise explicitly set (see usage for precedence).

### Example

Code:
```go
package main

import (
        "flag"
        "fmt"

        "github.com/jamiealquiza/envy"
)

func main() {
        var address = flag.String("address", "127.0.0.1", "Some random address")
        var port = flag.String("port", "8131", "Some random port")

        envy.Parse("MYAPP") // looks for MYAPP_ADDRESS & MYAPP_PORT
        flag.Parse()

        fmt.Println(*address)
        fmt.Println(*port)
}
```

Output:
```sh
# Prints flag defaults
% ./example
127.0.0.1
8131

# Flag defaults overridden
% MYAPP_ADDRESS="0.0.0.0" MYAPP_PORT="9080" ./example
0.0.0.0
9080
```

### Usage

**Variable precedence:**

Envy results in the following order of precedence, each item overwriting the previous:
`flag default` -> `Envy generated env var` -> `flag set at the CLI`.

Results referencing the example code:
- `./example` will result in `port` being set to `8131`
- `MYAPP_PORT=5678 ./example` will result in `port` being set to `5678`
- `MYAPP_PORT=5678 ./example -port=1234` will result in `port` being set to `1234`


**Env vars in help output:**

Envy can update your app help output so that it includes the environment variable generated/referenced for each flag. This is done by calling `envy.Parse()` before `flag.Parse()`.

The above example:
```
Usage of ./example:
  -address string
        Some random address [MYAPP_ADDRESS] (default "127.0.0.1")
  -port string
        Some random port [MYAPP_PORT] (default "8131")
```

 If this isn't desired, simply call `envy.Parse()` after `flag.Parse()`:
```go
// ...
	flag.Parse()
        envy.Parse("MYAPP") // looks for MYAPP_ADDRESS & MYAPP_PORT
// ...
```

```
Usage of ./example:
  -address string
        Some random address (default "127.0.0.1")
  -port string
        Some random port (default "8131")
```

**Satisfying types:**

Environment variables should be defined using a type that satisfies the respective type in your Go application's flag. For example:
- `string` -> `APP_ASTRINGVAR="someString"`
- `int` -> `APP_ANINTVAR=42`
- `bool` -> `APP_ABOOLVAR=true`

**Side effects:**

Setting a flag through an Envy generated environment variable will have the same effects on the default `flag.CommandLine` as if the flag were set via the command line. This only affect users that may rely on `flag.CommandLine` methods that make distinctions between set and to-be set flags (such as the `Visit` method).
