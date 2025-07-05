local config = {
  baseUrl: "https://httpbin.org",
  timeout: 30,
  retries: 3,
};

local endpoints = {
  get: config.baseUrl + "/get",
  post: config.baseUrl + "/post", 
  status: function(code) config.baseUrl + "/status/" + code,
};

{
  name: "jsonnet_http_advanced",
  on: "cli.manual",
  vars: config,
  steps: [
    {
      id: "test_get",
      use: "http.fetch",
      with: {
        url: endpoints.get,
        method: "GET"
      }
    },
    {
      id: "test_post",
      use: "http.fetch", 
      with: {
        url: endpoints.post,
        method: "POST",
        body: {
          message: "Hello from Jsonnet!",
          timestamp: "2024-01-01T00:00:00Z",
          config: config
        }
      }
    },
    {
      id: "summary",
      use: "core.echo",
      with: {
        text: std.join('\n', [
          "=== HTTP Test Results ===",
          "GET endpoint: " + endpoints.get,
          "POST endpoint: " + endpoints.post,
          "Base URL: " + config.baseUrl,
          "Configuration: " + std.toString(config)
        ])
      }
    }
  ]
}