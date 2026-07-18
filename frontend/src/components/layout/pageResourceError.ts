export const isPageResourceLoadError = (error: Error) => (
  /chunkloaderror|loading chunk [^ ]+ failed|failed to fetch dynamically imported module|importing a module script failed/i.test(error.message)
);
