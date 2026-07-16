import React from 'react';
import { useVirtualizer } from '@tanstack/react-virtual';
import { ChevronLeft, ChevronRight, Search } from 'lucide-react';

interface Header {
  key: string;
  label: string;
  align?: 'left' | 'center' | 'right';
}

interface Tab {
  id: string;
  label: string;
}

interface Pagination {
  currentPage: number;
  totalPages: number;
  totalItems: number;
  itemsPerPage: number;
  onPageChange: (page: number) => void;
}

interface DataTableProps<T> {
  title?: string;
  headers: Header[];
  data: T[];
  renderRow: (item: T, index: number) => React.ReactNode;
  renderCard?: (item: T, index: number) => React.ReactNode;
  pagination?: Pagination;
  searchPlaceholder?: string;
  searchText?: string;
  onSearchChange?: (text: string) => void;
  tabs?: Tab[];
  activeTab?: string;
  onTabChange?: (tabId: string) => void;
  headerActions?: React.ReactNode;
  emptyText?: string;
  virtualized?: boolean;
  estimatedRowHeight?: number;
}

export function DataTable<T>({
  title,
  headers,
  data,
  renderRow,
  renderCard,
  pagination,
  searchPlaceholder = '搜索...',
  searchText,
  onSearchChange,
  tabs,
  activeTab,
  onTabChange,
  headerActions,
  emptyText = '暂无匹配数据',
  virtualized = false,
  estimatedRowHeight = 64,
}: DataTableProps<T>) {
  const showCards = Boolean(renderCard);
  const safeData = Array.isArray(data) ? data : [];
  const scrollRef = React.useRef<HTMLDivElement>(null);
  const useVirtualRows = virtualized && safeData.length > 80;
  const rowVirtualizer = useVirtualizer({
    count: safeData.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => estimatedRowHeight,
    overscan: 10,
    enabled: useVirtualRows,
  });
  const virtualRows = useVirtualRows ? rowVirtualizer.getVirtualItems() : [];
  const topPadding = useVirtualRows && virtualRows.length > 0 ? virtualRows[0].start : 0;
  const bottomPadding =
    useVirtualRows && virtualRows.length > 0
      ? Math.max(0, rowVirtualizer.getTotalSize() - virtualRows[virtualRows.length - 1].end)
      : 0;
  const rowsToRender: Array<{ key: React.Key; index: number; item: T }> = useVirtualRows
    ? virtualRows.map((row) => ({ key: row.key, index: row.index, item: safeData[row.index] }))
    : safeData.map((item, index) => ({ key: index, index, item }));
  const paginationPages = pagination ? visiblePages(pagination.currentPage, pagination.totalPages) : [];

  return (
    <div className="w-full min-w-0">
      <div className="flex flex-col gap-4 border-b border-slate-200/80 pb-4 lg:flex-row lg:items-center lg:justify-between">
        <div className="flex min-w-0 flex-col gap-3 sm:flex-row sm:items-center">
          {title && <h3 className="text-base font-bold tracking-tight text-slate-900">{title}</h3>}
          {onSearchChange !== undefined && (
            <div className="relative w-full sm:w-72">
              <Search className="absolute left-3.5 top-1/2 -translate-y-1/2 text-slate-400" size={15} />
              <input
                type="search"
                aria-label={searchPlaceholder}
                placeholder={searchPlaceholder}
                value={searchText || ''}
                onChange={(event) => onSearchChange(event.target.value)}
                className="h-10 w-full rounded-lg border border-slate-200 bg-white pl-9 pr-4 text-sm font-medium text-slate-700 shadow-sm placeholder:text-slate-400 focus:border-sky-500 focus:outline-none focus:ring-2 focus:ring-sky-500/10"
              />
            </div>
          )}
        </div>

        <div className="flex min-w-0 flex-col gap-3 sm:flex-row sm:items-center sm:justify-between lg:justify-end">
          {tabs && tabs.length > 0 && activeTab && onTabChange && (
            <div className="flex max-w-full items-center overflow-x-auto rounded-lg border border-slate-200/80 bg-slate-100 p-1">
              {tabs.map((tab) => {
                const isActive = activeTab === tab.id;
                return (
                  <button
                    type="button"
                    key={tab.id}
                    onClick={() => onTabChange(tab.id)}
                    className={`shrink-0 rounded-md px-3 py-1.5 text-xs font-semibold transition-colors ${
                      isActive
                        ? 'bg-white text-slate-900 shadow-sm ring-1 ring-slate-200/70'
                        : 'text-slate-500 hover:text-slate-800'
                    }`}
                  >
                    {tab.label}
                  </button>
                );
              })}
            </div>
          )}

          {headerActions}
        </div>
      </div>

      {showCards && (
        <div className="grid gap-3 py-4 md:hidden">
          {safeData.length > 0 ? (
            safeData.map((item, index) => <React.Fragment key={index}>{renderCard?.(item, index)}</React.Fragment>)
          ) : (
            <div className="rounded-xl border border-dashed border-slate-200 bg-slate-50/70 px-4 py-10 text-center text-sm font-medium text-slate-500">
              {emptyText}
            </div>
          )}
        </div>
      )}

      <div
        ref={scrollRef}
        className={`${showCards ? 'hidden overflow-x-auto md:block' : 'overflow-x-auto'} ${useVirtualRows ? 'max-h-[680px] overflow-y-auto' : ''}`}
      >
        <table className="w-full border-collapse text-left" aria-label={title || '数据表格'}>
          <thead className="sticky top-0 z-10">
            <tr className="border-b border-slate-200/80 bg-slate-50/95 backdrop-blur">
              {headers.map((header) => (
                <th
                  key={header.key}
                  scope="col"
                  className={`whitespace-nowrap px-5 py-3.5 text-xs font-bold text-slate-500 ${
                    header.align === 'center' ? 'text-center' : header.align === 'right' ? 'text-right' : 'text-left'
                  }`}
                >
                  {header.label}
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100">
            {safeData.length > 0 ? (
              <>
                {topPadding > 0 && (
                  <tr aria-hidden="true">
                    <td colSpan={headers.length} style={{ height: topPadding }} className="p-0" />
                  </tr>
                )}
                {rowsToRender.map((row) => (
                  <React.Fragment key={row.key}>{renderRow(row.item, row.index)}</React.Fragment>
                ))}
                {bottomPadding > 0 && (
                  <tr aria-hidden="true">
                    <td colSpan={headers.length} style={{ height: bottomPadding }} className="p-0" />
                  </tr>
                )}
              </>
            ) : (
              <tr>
                <td colSpan={headers.length} className="px-6 py-14 text-center text-sm font-medium text-slate-500">
                  {emptyText}
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      {pagination && (
        <div className="mt-2 flex flex-col gap-3 border-t border-slate-200/80 pt-4 sm:flex-row sm:items-center sm:justify-between">
          <span className="text-xs font-semibold text-slate-500">
            显示 {pagination.totalItems === 0 ? 0 : (pagination.currentPage - 1) * pagination.itemsPerPage + 1} -{' '}
            {Math.min(pagination.totalItems, pagination.currentPage * pagination.itemsPerPage)} 条，共{' '}
            {pagination.totalItems} 条
          </span>

          <div className="flex items-center gap-1.5">
            <button
              type="button"
              onClick={() => pagination.onPageChange(Math.max(1, pagination.currentPage - 1))}
              disabled={pagination.currentPage === 1}
              aria-label="上一页"
              className="flex h-9 w-9 items-center justify-center rounded-lg border border-slate-200 bg-white text-slate-600 transition-colors hover:bg-slate-50 disabled:opacity-40"
            >
              <ChevronLeft size={14} />
            </button>

            {paginationPages.map((page) => {
              const isActive = pagination.currentPage === page;
              return (
                <button
                  type="button"
                  key={page}
                  onClick={() => pagination.onPageChange(page)}
                  className={`h-9 w-9 rounded-lg text-xs font-bold transition-colors ${
                    isActive
                      ? 'bg-slate-900 text-white shadow-sm'
                      : 'border border-slate-200 bg-white text-slate-600 hover:bg-slate-50'
                  }`}
                >
                  {page}
                </button>
              );
            })}

            <button
              type="button"
              onClick={() => pagination.onPageChange(Math.min(pagination.totalPages, pagination.currentPage + 1))}
              disabled={pagination.currentPage === pagination.totalPages}
              aria-label="下一页"
              className="flex h-9 w-9 items-center justify-center rounded-lg border border-slate-200 bg-white text-slate-600 transition-colors hover:bg-slate-50 disabled:opacity-40"
            >
              <ChevronRight size={14} />
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

const visiblePages = (currentPage: number, totalPages: number) => {
  if (totalPages <= 7) {
    return Array.from({ length: totalPages }, (_, index) => index + 1);
  }
  const pages = new Set([1, totalPages, currentPage]);
  for (const page of [currentPage - 1, currentPage + 1]) {
    if (page > 1 && page < totalPages) pages.add(page);
  }
  if (currentPage <= 3) {
    pages.add(2);
    pages.add(3);
  }
  if (currentPage >= totalPages - 2) {
    pages.add(totalPages - 1);
    pages.add(totalPages - 2);
  }
  return Array.from(pages).sort((a, b) => a - b);
};
