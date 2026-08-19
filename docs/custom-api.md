[Jump to function definitions](#functions)

## Examples

The best way to get an idea of how the templates work would be with a bunch examples. Here are the most common use cases:

JSON response:

```json
{
  "title": "My Title",
  "content": "My Content",
}
```

To access the two fields in the JSON response, you would use the following:

```html
<div>{{ .JSON.String "title" }}</div>
<div>{{ .JSON.String "content" }}</div>
```

Output:

```html
<div>My Title</div>
<div>My Content</div>
```

<hr>

JSON response:

```json
{
  "author": "John Doe",
  "posts": [
    {
      "title": "My Title",
      "content": "My Content"
    },
    {
      "title": "My Title 2",
      "content": "My Content 2"
    }
  ]
}
```

To loop through the array of posts, you would use the following:

```html
{{ range .JSON.Array "posts" }}
  <div>{{ .String "title" }}</div>
  <div>{{ .String "content" }}</div>
{{ end }}
```

Output:

```html
<div>My Title</div>
<div>My Content</div>
<div>My Title 2</div>
<div>My Content 2</div>
```

Notice the missing `.JSON` when accessing the title and content, this is because the range function sets the context to the current array element.

If you want to access the top-level context within the range, you can use the following:

```html
{{ range .JSON.Array "posts" }}
  <div>{{ .String "title" }}</div>
  <div>{{ .String "content" }}</div>
  <div>{{ $.JSON.String "author" }}</div>
{{ end }}
```

Output:

```html
<div>My Title</div>
<div>My Content</div>
<div>John Doe</div>
<div>My Title 2</div>
<div>My Content 2</div>
<div>John Doe</div>
```

<hr>

JSON response:

```json
[
    "Apple",
    "Banana",
    "Cherry",
    "Watermelon"
]
```

Somewhat awkwardly, when the current context is a basic type that isn't an object, the way you specify its type is to use an empty string as the key. So, to loop through the array of strings, you would use the following:

```html
{{ range .JSON.Array "" }}
  <div>{{ .String "" }}</div>
{{ end }}
```

Output:

```html
<div>Apple</div>
<div>Banana</div>
<div>Cherry</div>
<div>Watermelon</div>
```

To access an item at a specific index, you could use the following:

```html
<div>{{ .JSON.String "0" }}</div>
```

Output:

```html
<div>Apple</div>
```

<hr>

JSON response:

```json
{
    "user": {
        "address": {
            "city": "New York",
            "state": "NY"
        }
    }
}
```

To easily access deeply nested objects, you can use the following dot notation:

```html
<div>{{ .JSON.String "user.address.city" }}</div>
<div>{{ .JSON.String "user.address.state" }}</div>
```

Output:

```html
<div>New York</div>
<div>NY</div>
```

Using indexes anywhere in the path is also supported:

```json
{
    "users": [
        {
            "name": "John Doe"
        },
        {
            "name": "Jane Doe"
        }
    ]
}
```

```html
<div>{{ .JSON.String "users.0.name" }}</div>
<div>{{ .JSON.String "users.1.name" }}</div>
```

Output:

```html
<div>John Doe</div>
<div>Jane Doe</div>
```

<hr>

JSON response:

```json
{
    "user": {
        "name": "John Doe",
        "age": 30
    }
}
```

To check if a field exists, you can use the following:

```html
{{ if .JSON.Exists "user.age" }}
  <div>{{ .JSON.Int "user.age" }}</div>
{{ else }}
  <div>Age not provided</div>
{{ end }}
```

Output:

```html
<div>30</div>
```

<hr>

JSON response:

```json
{
    "price": 100,
    "discount": 10
}
```

Calculations can be performed on either ints or floats. If both numbers are ints, an int will be returned, otherwise a float will be returned. If you try to divide by zero, 0 will be returned. If you provide non-numeric values, `NaN` will be returned.

```html
<div>{{ sub (.JSON.Int "price") (.JSON.Int "discount") }}</div>
```

Output:

```html
<div>90</div>
```

Other operations include `add`, `mul`, `div` and `mod`.

<hr>

JSON response:

```json
{
  "posts": [
    {
      "title": "Exploring the Depths of Quantum Computing",
      "date": "2023-10-27T10:00:00Z"
    },
    {
      "title": "A Beginner's Guide to Sustainable Living",
      "date": "2023-11-15T14:30:00+01:00"
    },
    {
      "title": "The Art of Baking Sourdough Bread",
      "date": "2023-12-03T08:45:22-08:00"
    }
  ]
}
```

To parse the date and display the relative time (e.g. 2h, 1d, etc), you would use the following:

```
{{ range .JSON.Array "posts" }}
  <div>{{ .String "title" }}</div>
  <div {{ .String "date" | parseTime "rfc3339" | toRelativeTime }}></div>
{{ end }}
```

The `parseTime` function takes two arguments: the layout of the date string and the date string itself. The layout can be one of the following: "RFC3339", "RFC3339Nano", "DateTime", "DateOnly", "TimeOnly" or a custom layout in Go's [date format](https://pkg.go.dev/time#pkg-constants).

Output:

```html
<div>Exploring the Depths of Quantum Computing</div>
<div data-dynamic-relative-time="1698400800"></div>

<div>A Beginner's Guide to Sustainable Living</div>
<div data-dynamic-relative-time="1700055000"></div>

<div>The Art of Baking Sourdough Bread</div>
<div data-dynamic-relative-time="1701621922"></div>
```

You don't have to worry about the internal implementation, this will then be dynamically populated by Glance on the client side to show the correct relative time.

The important thing to notice here is that the return value of `toRelativeTime` must be used as an attribute in an HTML tag, be it a `div`, `li`, `span`, etc.

<hr>

In some instances, you may want to know the status code of the response. This can be done using the following:

```html
{{ if eq .Response.StatusCode 200 }}
  <p>Success!</p>
{{ else }}
  <p>Failed to fetch data</p>
{{ end }}
```

You can also access the response headers:

```html
<div>{{ .Response.Header.Get "Content-Type" }}</div>
```

<hr>

JSON response:

```json
{"name": "Steve", "age": 30}
{"name": "Alex", "age": 25}
{"name": "John", "age": 35}
```

The above format is "[ndjson](https://docs.mulesoft.com/dataweave/latest/dataweave-formats-ndjson)" or "[JSON Lines](https://jsonlines.org/)", where each line is a separate JSON object. To parse this format, you must first disable the JSON validation check in your config, since by default the response is expected to be a single valid JSON object:

```yaml
- type: custom-api
  skip-json-validation: true
```

Then, to iterate over each object you can use `.JSONLines`:

```html
{{ range .JSONLines }}
  <p>{{ .String "name" }} is {{ .Int "age" }} years old</p>
{{ end }}
```

Output:

```html
<p>Steve is 30 years old</p>
<p>Alex is 25 years old</p>
<p>John is 35 years old</p>
```

For other ways of selecting data from a JSON Lines response, have a look at the docs for [tidwall/gjson](https://github.com/tidwall/gjson/tree/master?tab=readme-ov-file#json-lines). For example, to  get an array of all names, you can use the following:

```html
{{ range .JSON.Array "..#.name" }}
  <p>{{ .String "" }}</p>
{{ end }}
```

Output:

```html
<p>Steve</p>
<p>Alex</p>
<p>John</p>
```

<hr>

In some instances, you may need to make two consecutive API calls, where you use the result of the first call in the second call. To achieve this, you can make additional HTTP requests from within the template itself using the following syntax:

```yaml
- type: custom-api
  url: https://api.example.com/get-id-of-something
  template: |
    {{ $theID := .JSON.String "id" }}

    {{
      $something := newRequest (concat "https://api.example.com/something/" $theID)
        | withParameter "key" "value"
        | withHeader "Authorization" "Bearer token"
        | getResponse
    }}

    {{ $something.JSON.String "title" }}
```

Here, `$theID` gets retrieved from the result of the first API call and used in the second API call. The `newRequest` function creates a new request, and the `getResponse` function executes it. You can also use `withParameter` and `withHeader` to optionally add parameters and headers to the request.

If you need to make a request to a URL that requires dynamic parameters, you can omit the `url` property in the YAML and run the request entirely from within the template itself:

```yaml
- type: custom-api
  title: Events from the last 24h
  template: |
    {{
      $events := newRequest "https://api.example.com/events"
        | withParameter "after" (offsetNow "-24h" | formatTime "rfc3339")
        | getResponse
    }}

    {{ if eq $events.Response.StatusCode 200 }}
      {{ range $events.JSON.Array "events" }}
        <div>{{ .String "title" }}</div>
        <div {{ .String "date" | parseTime "rfc3339" | toRelativeTime }}></div>
      {{ end }}
    {{ else }}
      <p>Failed to fetch data: {{ $events.Response.Status }}</p>
    {{ end }}
```

*Note that you need to manually check for the correct status code.*

## Functions

The following functions are available on the `JSON` object:

- `String(key string) string`: Returns the value of the key as a string.
- `Int(key string) int`: Returns the value of the key as an integer.
- `Float(key string) float`: Returns the value of the key as a float.
- `Bool(key string) bool`: Returns the value of the key as a boolean.
- `Array(key string) []JSON`: Returns the value of the key as an array of `JSON` objects.
- `Exists(key string) bool`: Returns true if the key exists in the JSON object.

The following functions are available on the `Options` object:

- `StringOr(key string, default string) string`: Returns the value of the key as a string, or the default value if the key does not exist.
- `IntOr(key string, default int) int`: Returns the value of the key as an integer, or the default value if the key does not exist.
- `FloatOr(key string, default float) float`: Returns the value of the key as a float, or the default value if the key does not exist.
- `BoolOr(key string, default bool) bool`: Returns the value of the key as a boolean, or the default value if the key does not exist.
- `JSON(key string) JSON`: Returns the value of the key as a stringified `JSON` object, or throws an error if the key does not exist.

The following helper functions provided by Glance are available:

- `toFloat(i int) float`: Converts an integer to a float.
- `toInt(f float) int`: Converts a float to an integer.
- `toRelativeTime(t time.Time) template.HTMLAttr`: Converts Time to a relative time such as 2h, 1d, etc which dynamically updates. **NOTE:** the value of this function should be used as an attribute in an HTML tag, e.g. `<span {{ toRelativeTime .Time }}></span>`.
- `now() time.Time`: Returns the current time.
- `offsetNow(offset string) time.Time`: Returns the current time with an offset. The offset can be positive or negative and must be in the format "3h" "-1h" or "2h30m10s".
- `duration(str string) time.Duration`: Parses a string such as `1h`, `24h`, `5h30m`, etc into a `time.Duration`.
- `parseTime(layout string, s string) time.Time`: Parses a string into time.Time. The layout must be provided in Go's [date format](https://pkg.go.dev/time#pkg-constants). You can alternatively use these values instead of the literal format: "unix", "RFC3339", "RFC3339Nano", "DateTime", "DateOnly".
- `formatTime(layout string, s string) time.Time`: Formats a `time.Time` into a string. The layout uses the same format as `parseTime`.
- `parseLocalTime(layout string, s string) time.Time`: Same as the above, except in the absence of a timezone, it will use the local timezone instead of UTC.
- `parseRelativeTime(layout string, s string) time.Time`: A shorthand for `{{ .String "date" | parseTime "rfc3339" | toRelativeTime }}`.
- `add(a, b float) float`: Adds two numbers.
- `sub(a, b float) float`: Subtracts two numbers.
- `mul(a, b float) float`: Multiplies two numbers.
- `div(a, b float) float`: Divides two numbers.
- `mod(a, b int) int`: Remainder after dividing a by b (a % b).
- `formatApproxNumber(n int) string`: Formats a number to be more human-readable, e.g. 1000 -> 1k.
- `formatNumber(n float|int) string`: Formats a number with commas, e.g. 1000 -> 1,000.
- `trimPrefix(prefix string, str string) string`: Trims the prefix from a string.
- `trimSuffix(suffix string, str string) string`: Trims the suffix from a string.
- `trimSpace(str string) string`: Trims whitespace from a string on both ends.
- `replaceAll(old string, new string, str string) string`: Replaces all occurrences of a string in a string.
- `replaceMatches(pattern string, replacement string, str string) string`: Replaces all occurrences of a regular expression in a string.
- `findMatch(pattern string, str string) string`: Finds the first match of a regular expression in a string.
- `findSubmatch(pattern string, str string) string`: Finds the first submatch of a regular expression in a string.
- `sortByString(key string, order string, arr []JSON): []JSON`: Sorts an array of JSON objects by a string key in either ascending or descending order.
- `sortByInt(key string, order string, arr []JSON): []JSON`: Sorts an array of JSON objects by an integer key in either ascending or descending order.
- `sortByFloat(key string, order string, arr []JSON): []JSON`: Sorts an array of JSON objects by a float key in either ascending or descending order.
- `sortByTime(key string, layout string, order string, arr []JSON): []JSON`: Sorts an array of JSON objects by a time key in either ascending or descending order. The format must be provided in Go's [date format](https://pkg.go.dev/time#pkg-constants).
- `concat(strings ...string) string`: Concatenates multiple strings together.
- `unique(key string, arr []JSON) []JSON`: Returns a unique array of JSON objects based on the given key.
- `percentChange(current float, previous float) float`: Calculates the percentage change between two numbers.
- `startOfDay(t time.Time) time.Time`: Returns the start of the day for a given time.
- `endOfDay(t time.Time) time.Time`: Returns the end of the day for a given time.

The following helper functions provided by Go's `text/template` are available:

- `eq(a, b any) bool`: Compares two values for equality.
- `ne(a, b any) bool`: Compares two values for inequality.
- `lt(a, b any) bool`: Compares two values for less than.
- `le(a, b any) bool`: Compares two values for less than or equal to.
- `gt(a, b any) bool`: Compares two values for greater than.
- `ge(a, b any) bool`: Compares two values for greater than or equal to.
- `and(args ...bool) bool`: Returns true if **all** arguments are true; accepts two or more boolean values.
- `or(args ...bool) bool`: Returns true if **any** argument is true; accepts two or more boolean values.
- `not(a bool) bool`: Returns the opposite of the value.
- `index(a any, b int) any`: Returns the value at the specified index of an array.
- `len(a any) int`: Returns the length of an array.
- `printf(format string, a ...any) string`: Returns a formatted string.
