type IToastSuccess = (message: string) => void;
type IToastError = (
  error: Error | string,
  displayReport?: boolean,
  timeout?: number
) => void;
