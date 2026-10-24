// Pure-math pagination for the provider detail connections list.
// Used by src/app/(dashboard)/dashboard/providers/[id]/page.js
// and guarded by tests/unit/provider-connections-pagination.test.js.

export const CONNECTIONS_PER_PAGE = 10;

export function computeConnectionPagination(connections = [], page = 1) {
  const list = Array.isArray(connections) ? connections : [];
  const totalItems = list.length;
  const totalPages = Math.max(1, Math.ceil(totalItems / CONNECTIONS_PER_PAGE));
  const numeric = Math.floor(Number(page));
  const safePage = Number.isFinite(numeric) && numeric > 0 ? numeric : 1;
  const currentPage = Math.min(safePage, totalPages);
  const start = (currentPage - 1) * CONNECTIONS_PER_PAGE;
  const items = list.slice(start, start + CONNECTIONS_PER_PAGE);
  return { currentPage, totalPages, totalItems, start, items, pageSize: CONNECTIONS_PER_PAGE };
}
