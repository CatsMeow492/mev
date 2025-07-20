export const formatDistanceToNow = (date: Date, options?: { addSuffix?: boolean }): string => {
  const now = new Date();
  const diffInMs = now.getTime() - date.getTime();
  const diffInSeconds = Math.floor(diffInMs / 1000);
  const diffInMinutes = Math.floor(diffInSeconds / 60);
  const diffInHours = Math.floor(diffInMinutes / 60);
  const diffInDays = Math.floor(diffInHours / 24);

  let result: string;

  if (diffInSeconds < 60) {
    result = diffInSeconds === 1 ? '1 second' : `${diffInSeconds} seconds`;
  } else if (diffInMinutes < 60) {
    result = diffInMinutes === 1 ? '1 minute' : `${diffInMinutes} minutes`;
  } else if (diffInHours < 24) {
    result = diffInHours === 1 ? '1 hour' : `${diffInHours} hours`;
  } else {
    result = diffInDays === 1 ? '1 day' : `${diffInDays} days`;
  }

  return options?.addSuffix ? `${result} ago` : result;
}; 