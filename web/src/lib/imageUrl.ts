import type { Image } from './api';

export function imageUrl(img: Image): string {
  if (img.file_path.startsWith('images/')) {
    return '/assets/images/' + img.file_path.substring('images/'.length);
  }
  if (img.file_path.startsWith('uploads/')) {
    return '/assets/uploads/' + img.file_path.substring('uploads/'.length);
  }
  return '/assets/' + img.file_path;
}
