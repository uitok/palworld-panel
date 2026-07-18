import React, { useEffect } from 'react';
import { createPortal } from 'react-dom';

export const ModalPortal: React.FC<React.PropsWithChildren> = ({ children }) => {
  useEffect(() => {
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    return () => {
      document.body.style.overflow = previousOverflow;
    };
  }, []);

  return createPortal(children, document.body);
};
