{
  name: "http_request_example",
  on: "cli.manual",
  vars: {
    URL: "https://httpbin.org/get"
  },
  steps: [
    {
      id: "fetch",
      use: "http.fetch",
      with: {
        url: "{{ vars.URL }}"
      }
    },
    {
      id: "print",
      use: "core.echo",
      with: {
        text: "Fetched {{ fetch.status }} from {{ vars.URL }}"
      }
    }
  ]
}