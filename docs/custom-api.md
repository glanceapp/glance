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

Calculations can be performed, however all numbers must be converted to floats first if they are not already:

```html
<div>{{ sub (.JSON.Int "price" | toFloat) (.JSON.Int "discount" | toFloat) }}</div>
```

Output:

```html
<div>90</div>
```

Other operations include `add`, `mul`, and `div`.

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

## Functions

The following functions are available on the `JSON` object:

- `String(key string) string`: Returns the value of the key as a string.
- `Int(key string) int`: Returns the value of the key as an integer.
- `Float(key string) float`: Returns the value of the key as a float.
- `Bool(key string) bool`: Returns the value of the key as a boolean.
- `Array(key string) []JSON`: Returns the value of the key as an array of `JSON` objects.
- `Exists(key string) bool`: Returns true if the key exists in the JSON object.

The following helper functions provided by Glance are available:

- `toFloat(i int) float`: Converts an integer to a float.
- `toInt(f float) int`: Converts a float to an integer.
- `add(a, b float) float`: Adds two numbers.
- `sub(a, b float) float`: Subtracts two numbers.
- `mul(a, b float) float`: Multiplies two numbers.
- `div(a, b float) float`: Divides two numbers.
- `formatApproxNumber(n int) string`: Formats a number to be more human-readable, e.g. 1000 -> 1k.
- `formatNumber(n float|int) string`: Formats a number with commas, e.g. 1000 -> 1,000.

The following helper functions provided by Go's `text/template` are available:

- `eq(a, b any) bool`: Compares two values for equality.
- `ne(a, b any) bool`: Compares two values for inequality.
- `lt(a, b any) bool`: Compares two values for less than.
- `lte(a, b any) bool`: Compares two values for less than or equal to.
- `gt(a, b any) bool`: Compares two values for greater than.
- `gte(a, b any) bool`: Compares two values for greater than or equal to.
- `and(a, b bool) bool`: Returns true if both values are true.
- `or(a, b bool) bool`: Returns true if either value is true.
- `not(a bool) bool`: Returns the opposite of the value.
- `index(a any, b int) any`: Returns the value at the specified index of an array.
- `len(a any) int`: Returns the length of an array.
- `printf(format string, a ...any) string`: Returns a formatted string.
