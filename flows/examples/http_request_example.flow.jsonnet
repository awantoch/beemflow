local vars = {
  URL: "https://httpbin.org/get"
};

{
  name: "http_request_example",
  on: "cli.manual",
  vars: vars,
  steps: [
    {
      id: "fetch",
      use: "http.fetch",
      with: {
        url: vars.URL
      }
    },
    {
      id: "print",
      use: "core.echo",
      with: {
        text: "Fetched from " + vars.URL
      }
    }
  ]
}