import { useContext } from 'react';
import { ServerStoreContext } from './serverStoreContext';

export const useServerStore = () => {
  const context = useContext(ServerStoreContext);
  if (context === undefined) {
    throw new Error('useServerStore must be used within a ServerStoreProvider');
  }
  return context;
};
