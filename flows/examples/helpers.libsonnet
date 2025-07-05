// Reusable Jsonnet helpers for example flows

local mkEcho(id, text) = {
  id: id,
  use: "core.echo",
  with: { text: text },
};

{
  mkEcho: mkEcho,
}