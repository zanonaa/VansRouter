export default {
  id: "devin-cli",
  alias: "dv",
  aliases: ["devin"],
  uiAlias: "dv",
  display: {
    name: "Devin CLI",
    icon: "smart_toy",
    color: "#6366F1",
    textIcon: "DV",
    website: "https://devin.ai",
    notice: {
      signupUrl: "https://cli.devin.ai",
      text: "Install the Devin CLI and run `devin auth login` first. Spawns the `devin` binary via ACP/stdio — no API key field.",
    },
  },
  category: "free",
  hasFree: true,
  authType: "none",
  noAuth: true,
  authModes: ["none"],
  transport: {
    baseUrl: "devin://acp/stdio",
    format: "openai",
  },
  // Model validity belongs to the installed Devin CLI/ACP, not this static registry.
  models: [],
};
