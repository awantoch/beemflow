local h = import "helpers.libsonnet";

{
  name: "jsonnet_fanout",
  on: "cli.manual",

  vars: {
    items: ["Moon", "Ocean", "Mountain"],
  },

  steps: [
    {
      id: "fanout",
      parallel: true,
      steps: [
        h.mkEcho("echo_" + item, "Hello " + item)
        for item in $.vars.items
      ],
    },
  ],
}