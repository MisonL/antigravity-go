import { useCallback, useState, type Dispatch, type SetStateAction } from 'react';

interface AsyncResourceConfig<T> {
  initialLoading?: boolean;
  initialValue: T;
}

interface AsyncRunOptions<T> {
  clearError?: boolean;
  onError?: (error: unknown) => string;
  onSuccess?: (value: T) => void | Promise<void>;
}

export interface AsyncResourceState<T> {
  data: T;
  error: string;
  loading: boolean;
  run: (loader: () => Promise<T>, options?: AsyncRunOptions<T>) => Promise<T | undefined>;
  setData: Dispatch<SetStateAction<T>>;
  setError: Dispatch<SetStateAction<string>>;
}

export function useAsyncResource<T>({
  initialLoading = false,
  initialValue,
}: AsyncResourceConfig<T>): AsyncResourceState<T> {
  const [data, setData] = useState(initialValue);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(initialLoading);

  const run = useCallback(async (
    loader: () => Promise<T>,
    options: AsyncRunOptions<T> = {},
  ): Promise<T | undefined> => {
    if (options.clearError !== false) {
      setError('');
    }
    setLoading(true);

    try {
      const value = await loader();
      setData(value);
      if (options.onSuccess) {
        await options.onSuccess(value);
      }
      return value;
    } catch (error) {
      if (options.onError) {
        setError(options.onError(error));
        return undefined;
      }
      throw error;
    } finally {
      setLoading(false);
    }
  }, []);

  return {
    data,
    error,
    loading,
    run,
    setData,
    setError,
  };
}
