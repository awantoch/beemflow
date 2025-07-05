local base = "https://httpbin.org";
local path = "/get";
local buildURL(b, p) = b + p;

{
  name: "http_request_example",
  on: "cli.manual",
  vars: {
    URL: buildURL(base, path),
  },
  steps: [
    {
      id: "fetch",
      use: "http.fetch",
      with: {
        url: $.vars.URL,
      },
    },
    {
      id: "print",
      use: "core.echo",
      with: {
        text: "Fetched {{ fetch.status }} from " + $.vars.URL,
      },
    },
  ],
}