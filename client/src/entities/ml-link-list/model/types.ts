/**
 * ML Link List entity types.
 * Central source of truth for all ML link list related types.
 */

/**
 * Represents a Mercado Livre link list.
 */
export interface MlLinkList {
  id: string;
  user_id: string;
  name: string;
  description: string | null;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

/**
 * Metadata for a link list item preview.
 */
export interface MlLinkListItemMetadata {
  preview?: {
    title?: string;
    price?: number;
    thumbnail?: string;
  };
}

/**
 * Represents an item in a Mercado Livre link list.
 */
export interface MlLinkListItem {
  id: string;
  list_id: string;
  origin_url: string;
  order_index: number;
  is_active: boolean;
  consumed_at: string | null;
  product_title: string | null;
  product_price: number | null;
  product_image_url: string | null;
  metadata: MlLinkListItemMetadata | null;
  info: string | null;
  created_at: string;
  updated_at: string;
}

/**
 * Response from the preview API.
 */
export interface PreviewResponse {
  title?: string | null;
  price?: number | null;
  thumbnail?: string | null;
}

/**
 * State for displaying the last preview.
 */
export interface PreviewState {
  url: string;
  title?: string | null;
  price?: number | null;
  thumbnail?: string | null;
}

/**
 * Form state for creating/editing a list.
 */
export interface ListForm {
  name: string;
  description: string;
  is_active: boolean;
}
