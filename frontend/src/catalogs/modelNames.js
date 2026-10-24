// Catalog lookup only: remove the UI's model(level) selection suffix.
// This never transforms a request body or selects an upstream wire format.
export function stripThinkingSuffix(model) {
  if (typeof model !== "string") return model;
  const match = model.match(/^(.*)\([^()]+\)\s*$/);
  return match ? match[1].trim() : model;
}

// Availability metadata for the level picker, not a request-field builder.
export function supportsKiroThinkingLevels(model) {
  if (typeof model !== "string") return false;
  const normalized = model.toLowerCase().replace(/-/g, ".");
  if (/(?:^|[/.])gpt[/.]5[/.]6(?:[/.]|$)/.test(normalized)) return true;
  const match = normalized.match(/(?:^|[/.])claude(?:[/.][a-z]+)*[/.](\d+)(?:[/.](\d+))?(?:[/.]|$)/);
  if (!match) return false;
  const major = Number(match[1]);
  const minor = match[2] === undefined ? null : Number(match[2]);
  return !(major < 4 || (major === 4 && (minor === null || minor <= 5 || minor >= 1000)));
}
