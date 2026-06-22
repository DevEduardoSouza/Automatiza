import { useAuth } from '@/contexts';
import type { ListForm, MlLinkList, MlLinkListItem, PreviewState } from '@/entities/ml-link-list';
import { useMercadoLivreConnect } from '@/hooks/use-mercadolivre-connect';
import { useMlLinksConnect } from '@/hooks/use-ml-links-connect';
import { useShopeeConnect } from '@/hooks/use-shopee-connect';
import { useToast } from '@/hooks/use-toast';
import { captureHookError } from '@/lib/sentry';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';

/**
 * Normalizes a URL by removing the hash fragment.
 */
function normalizeUrl(raw: string): string {
  const trimmed = String(raw || '').trim();
  const u = new URL(trimmed);
  u.hash = '';
  return u.toString();
}

/**
 * Aceita qualquer URL http(s) válida. A conversão automática para link de
 * afiliado só roda para Shopee no backend — pra outras lojas (ML inclusive,
 * Amazon, Magalu, etc) o user é responsável por colar o link já com a tag
 * dele de afiliado, e o sistema dispara o URL como veio.
 */
function isValidHttpUrl(raw: string): boolean {
  try {
    const u = new URL(String(raw || '').trim());
    return u.protocol === 'http:' || u.protocol === 'https:';
  } catch {
    return false;
  }
}

/**
 * Faz o parse de um CSV inteiro (RFC 4180) em registros de campos, respeitando
 * campos entre aspas duplas que podem conter o separador, aspas escapadas ("")
 * E quebras de linha. Isso é essencial pra coluna "info": uma descrição de
 * produto multi-linha (benefícios, indicação, etc) é exportada pelo Excel/
 * Sheets entre aspas com \n literais dentro — um split por linha quebraria.
 *
 * Aceita CRLF e LF. Uma quebra de linha FORA de aspas encerra o registro;
 * dentro de aspas ela faz parte do valor do campo.
 */
function parseCsvRecords(text: string, sep: string): string[][] {
  const records: string[][] = [];
  let row: string[] = [];
  let field = '';
  let inQuotes = false;
  let started = false; // a célula/registro atual recebeu algum char?

  const endField = () => {
    row.push(field);
    field = '';
  };
  const endRow = () => {
    endField();
    records.push(row);
    row = [];
    started = false;
  };

  for (let i = 0; i < text.length; i++) {
    const ch = text[i];
    if (inQuotes) {
      if (ch === '"') {
        if (text[i + 1] === '"') {
          field += '"';
          i++;
        } else {
          inQuotes = false;
        }
      } else {
        field += ch;
      }
      continue;
    }
    if (ch === '"') {
      inQuotes = true;
      started = true;
    } else if (ch === sep) {
      endField();
      started = true;
    } else if (ch === '\r') {
      // ignora o CR do CRLF; o \n seguinte fecha o registro
    } else if (ch === '\n') {
      if (started || field !== '') endRow();
    } else {
      field += ch;
      started = true;
    }
  }
  // Último registro sem newline final.
  if (started || field !== '') endRow();
  return records;
}

/**
 * Faz o parse do arquivo de importação em linhas {url, info}. Detecta o
 * separador (',' ou ';' — Excel pt-BR exporta com ';') a partir da 1ª linha
 * física (o cabeçalho não tem aspas), pula um cabeçalho "URL" opcional, e
 * pareia a 1ª coluna (link) com a 2ª (texto de info, usado na variável {info}
 * do disparo). A info pode ser multi-linha (entre aspas na planilha).
 */
function parseImportRows(fileText: string): Array<{ url: string; info: string }> {
  const firstLine = fileText.split(/\r?\n/, 1)[0] ?? '';
  // Detecção de separador: usa o delimitador mais frequente na 1ª linha (URLs
  // raramente têm vírgula/ponto-e-vírgula; a célula de info, se tiver vírgula,
  // vem entre aspas).
  const sep =
    firstLine.split(';').length - 1 > firstLine.split(',').length - 1 ? ';' : ',';

  const records = parseCsvRecords(fileText, sep);
  const rows: Array<{ url: string; info: string }> = [];
  for (let idx = 0; idx < records.length; idx++) {
    const fields = records[idx];
    const url = (fields[0] || '').trim();
    // Pula o cabeçalho ("URL", "url", "link"...) — só na primeira linha.
    if (idx === 0 && !isValidHttpUrl(url)) continue;
    if (!url) continue;
    // Aceita `\n` literal como marcador de quebra de linha, pra quem prefere
    // escrever o info numa linha só (sem Alt+Enter / célula multi-linha). Quem
    // usa quebra real na célula também funciona — não há `\n` literal pra
    // converter. Os dois jeitos coexistem.
    const info = (fields[1] || '').replace(/\\n/g, '\n').trim();
    rows.push({ url, info });
  }
  return rows;
}

/** True quando o host casa com algum domínio do Mercado Livre. */
function isMlUrl(u: URL): boolean {
  const host = u.hostname.toLowerCase();
  if (host === 'meli.la') return true;
  return host.includes('mercadolivre.') || host.includes('mercadolibre.');
}

/** True quando o host casa com algum domínio da Shopee. */
function isShopeeUrl(u: URL): boolean {
  const host = u.hostname.toLowerCase();
  return host.includes('shopee.com') || host.includes('shp.ee');
}

interface UseMlLinkListsReturn {
  loadingLists: boolean;
  loadingItems: boolean;
  loadingMoreItems: boolean;
  lists: MlLinkList[];
  selectedListId: string;
  items: MlLinkListItem[];
  listDialogOpen: boolean;
  editingList: MlLinkList | null;
  listForm: ListForm;
  singleUrl: string;
  importing: boolean;
  lastPreview: PreviewState | null;
  selectedList: MlLinkList | null;
  availableCount: number;
  sentCount: number;
  totalItemCount: number;
  hasMoreItems: boolean;
  deleteListTarget: MlLinkList | null;
  deleteItemTarget: MlLinkListItem | null;
  resetConsumedDialogOpen: boolean;
  savingList: boolean;
  setSelectedListId: (id: string) => void;
  setListDialogOpen: (open: boolean) => void;
  setListForm: React.Dispatch<React.SetStateAction<ListForm>>;
  setSingleUrl: (url: string) => void;
  importFromFile: (file: File) => Promise<void>;
  loadLists: () => Promise<void>;
  loadMoreItems: () => Promise<void>;
  openCreateList: () => void;
  openEditList: (list: MlLinkList) => void;
  saveList: () => Promise<void>;
  requestDeleteList: (list: MlLinkList) => void;
  cancelDeleteList: () => void;
  confirmDeleteList: () => Promise<void>;
  requestDeleteItem: (item: MlLinkListItem) => void;
  cancelDeleteItem: () => void;
  confirmDeleteItem: () => Promise<void>;
  requestResetConsumed: () => void;
  cancelResetConsumed: () => void;
  confirmResetConsumed: () => Promise<void>;
  copyToClipboard: (text: string) => Promise<void>;
  addOneLink: () => Promise<void>;
  toggleItemActive: (item: MlLinkListItem) => Promise<void>;
  editItemInfo: (item: MlLinkListItem, info: string) => Promise<void>;
  moveItem: (item: MlLinkListItem, dir: 'up' | 'down') => Promise<void>;
}

/**
 * Converts ConnectRPC local type to entity type for MlLinkList.
 */
function toEntityMlLinkList(local: import('./use-ml-links-connect').LocalMlLinkList): MlLinkList {
  return {
    id: local.id,
    user_id: local.user_id,
    name: local.name,
    description: local.description ?? null,
    is_active: local.is_active,
    created_at: local.created_at,
    updated_at: local.updated_at,
  };
}

/**
 * Converts ConnectRPC local type to entity type for MlLinkListItem.
 */
function toEntityMlLinkListItem(
  local: import('./use-ml-links-connect').LocalMlLinkListItem
): MlLinkListItem {
  let metadata: MlLinkListItem['metadata'] = null;
  if (local.metadata) {
    try {
      metadata = JSON.parse(local.metadata) as MlLinkListItem['metadata'];
    } catch (error: unknown) {
      captureHookError(error, 'useMlLinkLists', 'parseItemMetadata');
      metadata = null;
    }
  }

  return {
    id: local.id,
    list_id: local.list_id,
    origin_url: local.origin_url,
    order_index: local.order_index,
    is_active: local.is_active,
    consumed_at: local.consumed_at ?? null,
    product_title: local.product_title ?? null,
    product_price: local.product_price ?? null,
    product_image_url: local.product_image_url ?? null,
    metadata,
    info: local.info ?? null,
    created_at: local.created_at,
    updated_at: local.updated_at,
  };
}

/**
 * Hook for ML Link Lists page logic.
 * Manages Mercado Livre link lists with items and confirmation dialogs.
 * Uses ConnectRPC for all backend operations.
 */
export function useMlLinkLists(): UseMlLinkListsReturn {
  const { t } = useTranslation();
  const { user } = useAuth();
  const { toast } = useToast();
  const { preview: mlPreview } = useMercadoLivreConnect();
  const { getProductPreview: shopeePreview } = useShopeeConnect();

  // ConnectRPC hook for all ML Links operations
  const mlLinksConnect = useMlLinksConnect();

  const [loadingItems, setLoadingItems] = useState(false);
  const [loadingMoreItems, setLoadingMoreItems] = useState(false);
  const [selectedListId, setSelectedListId] = useState<string>('');
  const [items, setItems] = useState<MlLinkListItem[]>([]);
  const [nextPageToken, setNextPageToken] = useState<string>('');
  const [totalItemCount, setTotalItemCount] = useState<number>(0);
  const [serverAvailableCount, setServerAvailableCount] = useState<number>(0);
  const [serverSentCount, setServerSentCount] = useState<number>(0);
  const [hasMoreItems, setHasMoreItems] = useState<boolean>(false);

  const [listDialogOpen, setListDialogOpen] = useState(false);
  const [editingList, setEditingList] = useState<MlLinkList | null>(null);
  const [listForm, setListForm] = useState<ListForm>({
    name: '',
    description: '',
    is_active: true,
  });

  const [singleUrl, setSingleUrl] = useState('');
  const [importing, setImporting] = useState(false);

  const [lastPreview, setLastPreview] = useState<PreviewState | null>(null);

  const [deleteListTarget, setDeleteListTarget] = useState<MlLinkList | null>(null);
  const [deleteItemTarget, setDeleteItemTarget] = useState<MlLinkListItem | null>(null);
  const [resetConsumedDialogOpen, setResetConsumedDialogOpen] = useState(false);

  // Map ConnectRPC lists to entity types
  const lists = useMemo(() => mlLinksConnect.lists.map(toEntityMlLinkList), [mlLinksConnect.lists]);

  const loadingLists = mlLinksConnect.loadingLists;

  const selectedList = useMemo(
    () => lists.find((l) => l.id === selectedListId) || null,
    [lists, selectedListId]
  );

  // Backend agrega COUNT(*) FILTER em uma query só e retorna no
  // paginated response. Contar localmente seria errado para listas
  // maiores que o pageSize (caso real: lista de 362 itens mostrando
  // "Disponíveis: 100" porque só 100 estavam carregados).
  const availableCount = serverAvailableCount;
  const sentCount = serverSentCount;

  const selectedListIdRef = useRef(selectedListId);
  selectedListIdRef.current = selectedListId;

  const refetchListsRef = useRef(mlLinksConnect.refetchLists);
  refetchListsRef.current = mlLinksConnect.refetchLists;

  const loadLists = useCallback(async () => {
    if (!user) return;
    await refetchListsRef.current();
  }, [user]);

  const listMlLinkListItemsPaginatedRef = useRef(mlLinksConnect.listMlLinkListItemsPaginated);
  listMlLinkListItemsPaginatedRef.current = mlLinksConnect.listMlLinkListItemsPaginated;

  const loadItems = useCallback(
    async (listId: string, _reset = true) => {
      if (!user || !listId) return;
      setLoadingItems(true);
      try {
        const result = await listMlLinkListItemsPaginatedRef.current({
          listId,
          pageSize: 50,
          pageToken: '',
        });
        setItems(result.items.map(toEntityMlLinkListItem));
        setNextPageToken(result.nextPageToken);
        setTotalItemCount(result.totalCount);
        setServerAvailableCount(result.availableCount);
        setServerSentCount(result.sentCount);
        setHasMoreItems(result.hasMore);
      } catch (e: unknown) {
        const errorMessage =
          e instanceof Error ? e.message : t('pages.mlLinkLists.errors.loadLinksMessage');
        toast({
          title: t('pages.mlLinkLists.errors.loadLinks'),
          description: errorMessage,
          variant: 'destructive',
        });
      } finally {
        setLoadingItems(false);
      }
    },
    [user, toast, t]
  );

  const loadMoreItems = useCallback(async () => {
    if (!user || !selectedListId || !hasMoreItems || loadingMoreItems) return;
    setLoadingMoreItems(true);
    try {
      const result = await listMlLinkListItemsPaginatedRef.current({
        listId: selectedListId,
        pageSize: 50,
        pageToken: nextPageToken,
      });
      setItems((prev) => [...prev, ...result.items.map(toEntityMlLinkListItem)]);
      setNextPageToken(result.nextPageToken);
      setServerAvailableCount(result.availableCount);
      setServerSentCount(result.sentCount);
      setHasMoreItems(result.hasMore);
    } catch (e: unknown) {
      const errorMessage =
        e instanceof Error ? e.message : t('pages.mlLinkLists.errors.loadLinksMessage');
      toast({
        title: t('pages.mlLinkLists.errors.loadLinks'),
        description: errorMessage,
        variant: 'destructive',
      });
    } finally {
      setLoadingMoreItems(false);
    }
  }, [user, selectedListId, hasMoreItems, loadingMoreItems, nextPageToken, toast, t]);

  const userIdRef = useRef(user?.id);
  userIdRef.current = user?.id;

  // Auto-select first list when lists are loaded
  useEffect(() => {
    if (!selectedListIdRef.current && lists.length > 0) {
      setSelectedListId(lists[0].id);
    }
  }, [lists]);

  useEffect(() => {
    if (selectedListId) {
      loadItems(selectedListId);
    } else {
      setItems([]);
    }
  }, [selectedListId, loadItems]);

  const openCreateList = useCallback(() => {
    setEditingList(null);
    setListForm({ name: '', description: '', is_active: true });
    setListDialogOpen(true);
  }, []);

  const openEditList = useCallback((l: MlLinkList) => {
    setEditingList(l);
    setListForm({
      name: l.name || '',
      description: l.description || '',
      is_active: l.is_active !== false,
    });
    setListDialogOpen(true);
  }, []);

  const saveList = useCallback(async () => {
    if (!user) return;
    const name = String(listForm.name || '').trim();
    if (!name) {
      toast({
        title: t('pages.mlLinkLists.errors.nameRequired'),
        description: t('pages.mlLinkLists.errors.nameRequiredMessage'),
        variant: 'destructive',
      });
      return;
    }
    try {
      if (editingList) {
        await mlLinksConnect.updateMlLinkList({
          id: editingList.id,
          name,
          description: String(listForm.description || '').trim() || undefined,
          is_active: listForm.is_active,
        });
      } else {
        const newList = await mlLinksConnect.createMlLinkList({
          name,
          description: String(listForm.description || '').trim() || undefined,
          is_active: listForm.is_active,
        });
        if (newList?.id) setSelectedListId(newList.id);
      }
      setListDialogOpen(false);
      setEditingList(null);
    } catch (e: unknown) {
      const errorMessage =
        e instanceof Error ? e.message : t('pages.mlLinkLists.errors.saveMessage');
      toast({
        title: t('pages.mlLinkLists.errors.save'),
        description: errorMessage,
        variant: 'destructive',
      });
    }
  }, [user, listForm, editingList, toast, t, mlLinksConnect]);

  const requestDeleteList = useCallback((list: MlLinkList) => {
    setDeleteListTarget(list);
  }, []);

  const cancelDeleteList = useCallback(() => {
    setDeleteListTarget(null);
  }, []);

  const confirmDeleteList = useCallback(async () => {
    if (!user || !deleteListTarget) return;
    try {
      await mlLinksConnect.deleteMlLinkList(deleteListTarget.id);
      if (selectedListId === deleteListTarget.id) setSelectedListId('');
    } catch (e: unknown) {
      const errorMessage =
        e instanceof Error ? e.message : t('pages.mlLinkLists.errors.deleteMessage');
      toast({
        title: t('pages.mlLinkLists.errors.delete'),
        description: errorMessage,
        variant: 'destructive',
      });
    } finally {
      setDeleteListTarget(null);
    }
  }, [user, deleteListTarget, selectedListId, toast, t, mlLinksConnect]);

  const requestDeleteItem = useCallback((item: MlLinkListItem) => {
    setDeleteItemTarget(item);
  }, []);

  const cancelDeleteItem = useCallback(() => {
    setDeleteItemTarget(null);
  }, []);

  const confirmDeleteItem = useCallback(async () => {
    if (!deleteItemTarget) return;
    try {
      await mlLinksConnect.deleteMlLinkListItem(deleteItemTarget.id);
      loadItems(deleteItemTarget.list_id);
    } catch (e: unknown) {
      const errorMessage =
        e instanceof Error ? e.message : t('pages.mlLinkLists.errors.removeMessage');
      toast({
        title: t('pages.mlLinkLists.errors.remove'),
        description: errorMessage,
        variant: 'destructive',
      });
    } finally {
      setDeleteItemTarget(null);
    }
  }, [deleteItemTarget, toast, t, loadItems, mlLinksConnect]);

  const requestResetConsumed = useCallback(() => {
    setResetConsumedDialogOpen(true);
  }, []);

  const cancelResetConsumed = useCallback(() => {
    setResetConsumedDialogOpen(false);
  }, []);

  const confirmResetConsumed = useCallback(async () => {
    if (!selectedListId) return;
    try {
      await mlLinksConnect.resetConsumedItems(selectedListId);
      loadItems(selectedListId);
    } catch (e: unknown) {
      const errorMessage =
        e instanceof Error ? e.message : t('pages.mlLinkLists.errors.resetMessage');
      toast({
        title: t('pages.mlLinkLists.errors.reset'),
        description: errorMessage,
        variant: 'destructive',
      });
    } finally {
      setResetConsumedDialogOpen(false);
    }
  }, [selectedListId, toast, t, loadItems, mlLinksConnect]);

  const copyToClipboard = useCallback(
    async (text: string) => {
      try {
        await navigator.clipboard.writeText(text);
        toast({ title: t('pages.mlLinkLists.success.copied') });
      } catch (error: unknown) {
        captureHookError(error, 'useMlLinkLists', 'copyToClipboard');
      }
    },
    [toast, t]
  );

  const addOneLink = useCallback(async () => {
    if (!user || !selectedListId) return;
    const raw = String(singleUrl || '').trim();
    if (!raw) return;
    if (!isValidHttpUrl(raw)) {
      toast({
        title: t('pages.mlLinkLists.errors.invalidLink'),
        description: t('pages.mlLinkLists.errors.invalidLinkMessage'),
        variant: 'destructive',
      });
      return;
    }

    let cleanUrl = raw;
    try {
      cleanUrl = normalizeUrl(raw);
    } catch (error: unknown) {
      captureHookError(error, 'useMlLinkLists', 'normalizeUrl');
    }

    let preview: PreviewState | null = null;
    // Detecção da loja via host. ML e Shopee têm preview (Shopee via API
    // própria; ML via scraper). Outras lojas entram passthrough: o item é
    // gravado com o URL como veio e dispara sem preview server-side — o
    // user pode preencher título/preço/imagem manualmente depois se quiser.
    let parsed: URL | null = null;
    try {
      parsed = new URL(cleanUrl);
    } catch {
      parsed = null;
    }
    const isML = parsed ? isMlUrl(parsed) : false;
    const isShopee = parsed ? isShopeeUrl(parsed) : false;

    if (isML) {
      try {
        const previewData = await mlPreview(cleanUrl);
        if (previewData) {
          preview = {
            url: cleanUrl,
            title: previewData.title ?? null,
            price: previewData.price ?? null,
            thumbnail: previewData.imageUrl ?? null,
          };
          setLastPreview(preview);
        }
      } catch (e: unknown) {
        const errorMessage =
          e instanceof Error ? e.message : t('pages.mlLinkLists.errors.previewUnavailableMessage');
        toast({
          title: t('pages.mlLinkLists.errors.previewUnavailable'),
          description: errorMessage,
          variant: 'destructive',
        });
      }
    } else if (isShopee) {
      try {
        const previewData = await shopeePreview(cleanUrl);
        if (previewData) {
          preview = {
            url: cleanUrl,
            title: previewData.title,
            price: previewData.price,
            thumbnail: previewData.imageUrl,
          };
          setLastPreview(preview);
        }
      } catch {
        // Shopee preview is best-effort; item will still be added
      }
    }
    // else: outra loja — sem preview automático, item é adicionado direto.

    try {
      const nextOrder =
        items.length > 0 ? Math.max(...items.map((i) => i.order_index || 0)) + 1 : 0;

      const metadata = preview
        ? JSON.stringify({
            preview: { title: preview.title, price: preview.price, thumbnail: preview.thumbnail },
          })
        : undefined;

      await mlLinksConnect.createMlLinkListItem({
        list_id: selectedListId,
        origin_url: cleanUrl,
        order_index: nextOrder,
        is_active: true,
        product_title: preview?.title ?? undefined,
        product_price: preview?.price ?? undefined,
        product_image_url: preview?.thumbnail ?? undefined,
        metadata,
      });
      setSingleUrl('');
      loadItems(selectedListId);
    } catch (e: unknown) {
      const errorMessage =
        e instanceof Error ? e.message : t('pages.mlLinkLists.errors.addLinkMessage');
      toast({
        title: t('pages.mlLinkLists.errors.addLink'),
        description: errorMessage,
        variant: 'destructive',
      });
    }
  }, [user, selectedListId, singleUrl, items, toast, t, loadItems, mlPreview, mlLinksConnect]);

  const importFromFile = useCallback(async (file: File) => {
    if (!user || !selectedListId) {
      toast({
        title: t('pages.mlLinkLists.errors.import'),
        description: 'Selecione uma lista antes de importar.',
        variant: 'destructive',
      });
      return;
    }
    setImporting(true);
    try {
      const fileText = await file.text();
      const rows = parseImportRows(fileText);

      console.log('[import] rows parsed:', rows.length);

      // Preserva ordem da planilha e mantém duplicatas — não usa Set.
      // A planilha é a fonte da verdade: se o cliente repetiu o mesmo
      // link 3x, queremos 3 itens. Validação só elimina linhas que não
      // são URLs http(s) parseáveis. Filtramos os pares url+info juntos
      // pra manter o alinhamento por índice (info da linha descartada cai
      // junto com a URL).
      const validRows = rows.filter((r) => isValidHttpUrl(r.url));
      const validated = validRows.map((r) => {
        try {
          return normalizeUrl(r.url);
        } catch {
          return r.url;
        }
      });
      const infos = validRows.map((r) => r.info);

      console.log('[import] valid URLs:', validated.length);

      if (validated.length === 0) {
        toast({
          title: t('pages.mlLinkLists.errors.noValidLinks'),
          description: t('pages.mlLinkLists.errors.noValidLinksMessage'),
          variant: 'destructive',
        });
        return;
      }

      const baseOrder =
        items.length > 0 ? Math.max(...items.map((i) => i.order_index || 0)) + 1 : 0;

      console.log('[import] sending', validated.length, 'URLs to backend');
      const result = await mlLinksConnect.bulkCreateMlLinkListItems(
        selectedListId,
        validated,
        baseOrder,
        infos
      );
      console.log('[import] result:', result);
      await loadItems(selectedListId);
      toast({
        title: t('pages.mlLinkLists.items.imported', { count: result.createdCount }),
      });

      // Avisa o cliente se alguma linha foi descartada — backend é
      // permissivo (qualquer http(s)), então só caem aqui linhas
      // realmente inválidas (não-http, malformadas, scheme estranho).
      if (result.skippedCount > 0) {
        const preview = result.skippedUrls.slice(0, 5).join(', ');
        const more = result.skippedUrls.length > 5 ? ` (+${result.skippedUrls.length - 5})` : '';
        toast({
          title: t('pages.mlLinkLists.items.skipped', { count: result.skippedCount }),
          description: `${preview}${more}`,
          variant: 'destructive',
        });
      }
    } catch (e: unknown) {
      console.error('[import] error:', e);
      const errorMessage =
        e instanceof Error ? e.message : t('pages.mlLinkLists.errors.importMessage');
      toast({
        title: t('pages.mlLinkLists.errors.import'),
        description: errorMessage,
        variant: 'destructive',
      });
    } finally {
      setImporting(false);
    }
  }, [user, selectedListId, items, toast, t, loadItems, mlLinksConnect]);

  const toggleItemActive = useCallback(
    async (it: MlLinkListItem) => {
      try {
        await mlLinksConnect.updateMlLinkListItem({
          id: it.id,
          is_active: !it.is_active,
        });
        loadItems(it.list_id);
      } catch (e: unknown) {
        const errorMessage =
          e instanceof Error ? e.message : t('pages.mlLinkLists.errors.toggleMessage');
        toast({
          title: t('pages.mlLinkLists.errors.toggle'),
          description: errorMessage,
          variant: 'destructive',
        });
      }
    },
    [toast, t, loadItems, mlLinksConnect]
  );

  const editItemInfo = useCallback(
    async (it: MlLinkListItem, info: string) => {
      try {
        await mlLinksConnect.updateMlLinkListItem({
          id: it.id,
          info,
        });
        loadItems(it.list_id);
      } catch (e: unknown) {
        const errorMessage =
          e instanceof Error ? e.message : t('pages.mlLinkLists.errors.updateItemFailed');
        toast({
          title: t('pages.mlLinkLists.errors.updateItem'),
          description: errorMessage,
          variant: 'destructive',
        });
      }
    },
    [toast, t, loadItems, mlLinksConnect]
  );

  const moveItem = useCallback(
    async (it: MlLinkListItem, dir: 'up' | 'down') => {
      const idx = items.findIndex((x) => x.id === it.id);
      const swapIdx = dir === 'up' ? idx - 1 : idx + 1;
      if (idx < 0 || swapIdx < 0 || swapIdx >= items.length) return;
      const other = items[swapIdx];
      try {
        const a = it.order_index;
        const b = other.order_index;

        // Use reorderItems RPC with a map of swapped order indices
        await mlLinksConnect.reorderItems(it.list_id, {
          [it.id]: b,
          [other.id]: a,
        });
        loadItems(it.list_id);
      } catch (e: unknown) {
        const errorMessage =
          e instanceof Error ? e.message : t('pages.mlLinkLists.errors.reorderMessage');
        toast({
          title: t('pages.mlLinkLists.errors.reorder'),
          description: errorMessage,
          variant: 'destructive',
        });
      }
    },
    [items, toast, t, loadItems, mlLinksConnect]
  );

  return {
    loadingLists,
    loadingItems,
    loadingMoreItems,
    lists,
    selectedListId,
    items,
    listDialogOpen,
    editingList,
    listForm,
    singleUrl,
    importing,
    lastPreview,
    selectedList,
    availableCount,
    sentCount,
    totalItemCount,
    hasMoreItems,
    deleteListTarget,
    deleteItemTarget,
    resetConsumedDialogOpen,
    savingList: mlLinksConnect.isCreatingList || mlLinksConnect.isUpdatingList,
    setSelectedListId,
    setListDialogOpen,
    setListForm,
    setSingleUrl,
    importFromFile,
    loadLists,
    loadMoreItems,
    openCreateList,
    openEditList,
    saveList,
    requestDeleteList,
    cancelDeleteList,
    confirmDeleteList,
    requestDeleteItem,
    cancelDeleteItem,
    confirmDeleteItem,
    requestResetConsumed,
    cancelResetConsumed,
    confirmResetConsumed,
    copyToClipboard,
    addOneLink,
    toggleItemActive,
    editItemInfo,
    moveItem,
  };
}
