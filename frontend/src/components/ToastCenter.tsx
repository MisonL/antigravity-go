import { memo } from 'react';
import { useAppDomain, useNotifications } from '../domains/AppDomainContext';

export const ToastCenter = memo(function ToastCenter() {
  const { t } = useAppDomain();
  const { dismissNotification, notifications } = useNotifications();

  if (notifications.length === 0) {
    return null;
  }

  return (
    <div className="toast-stack" aria-live="polite" aria-atomic="true">
      {notifications.map((item) => (
        <div key={item.id} className={`toast toast-${item.kind}`} role="status">
          <div className="toast-message">{item.message}</div>
          <button
            className="toast-close"
            onClick={() => dismissNotification(item.id)}
            type="button"
            aria-label={t('common.close')}
          >
            X
          </button>
        </div>
      ))}
    </div>
  );
});
