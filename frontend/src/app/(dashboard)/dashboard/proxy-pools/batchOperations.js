export function dedupeProxyEntries(entries, existingKeys) {
  const accepted = [];
  let skipped = 0;
  for (const entry of entries) {
    const key = `${entry.proxyUrl}|||`;
    if (existingKeys.has(key)) {
      skipped += 1;
      continue;
    }
    existingKeys.add(key);
    accepted.push(entry);
  }
  return { accepted, skipped };
}

export async function runProxyPoolBatch(items, operation, onProgress) {
  const results = [];
  for (const item of items) {
    try {
      results.push(await operation(item));
    } catch {
      results.push("fail");
    }
    onProgress?.(results.length, items.length);
  }
  return results;
}

export function countBatchResults(results) {
  return results.reduce((counts, result) => {
    counts[result] = (counts[result] || 0) + 1;
    return counts;
  }, {});
}
