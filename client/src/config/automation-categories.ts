/**
 * Configuration for automation categories and subcategories.
 * Labels are stored in i18n (config.stores.*, config.categories.*, config.subcategories.*).
 */

import type { StoreKey } from '@/types/automation';

/** Store configuration with label and i18n key */
interface StoreConfig {
  value: StoreKey;
  /** Display label (for backwards compatibility) */
  label: string;
  /** i18n key for localized label */
  i18nKey: string;
}

/** Subcategory item with label and i18n key */
interface SubcategoryItem {
  key: string;
  /** Display label (for backwards compatibility) */
  label: string;
  /** i18n key for localized label */
  i18nKey: string;
}

/** Subcategory group with name and i18n key */
interface SubcategoryGroup {
  /** Group name (for backwards compatibility) */
  name: string;
  /** i18n key for localized name */
  i18nKey: string;
  items: readonly SubcategoryItem[];
}

/**
 * Available stores with labels and i18n keys.
 * Labels are kept for API compatibility (backend expects exact values).
 * Use i18nKey for UI display with translations.
 */
export const STORES: StoreConfig[] = [
  { value: 'shopee', label: 'Shopee', i18nKey: 'config.stores.shopee' },
  { value: 'amazon', label: 'Amazon', i18nKey: 'config.stores.amazon' },
  { value: 'magalu', label: 'Magazine Luiza', i18nKey: 'config.stores.magalu' },
  { value: 'ml', label: 'Mercado Livre', i18nKey: 'config.stores.ml' },
  { value: 'shein', label: 'Shein', i18nKey: 'config.stores.shein' },
  { value: 'aliexpress', label: 'AliExpress', i18nKey: 'config.stores.aliexpress' },
  { value: 'awin', label: 'Awin', i18nKey: 'config.stores.awin' },
];

/** Category keys - used as identifiers, labels come from i18n */
export const CATEGORY_KEYS = [
  'ofertasGerais',
  'eletronicos',
  'eletrodomesticos',
  'moda',
  'homens',
  'casaDecoracao',
  'beleza',
  'esportes',
  'livros',
  'brinquedos',
  'infantil',
  'maternidade',
  'alimentos',
  'automotivo',
  'pets',
] as const;

export type CategoryKey = (typeof CATEGORY_KEYS)[number];

/**
 * Map from category key to display name.
 * These values are sent to the API and must match backend expectations.
 * For UI display, use CATEGORY_I18N_KEYS with translation function.
 */
export const CATEGORY_DISPLAY_NAMES: Record<CategoryKey, string> = {
  ofertasGerais: 'Ofertas gerais',
  eletronicos: 'Eletrônicos',
  eletrodomesticos: 'Eletrodomésticos',
  moda: 'Moda',
  homens: 'Homens',
  casaDecoracao: 'Casa e Decoração',
  beleza: 'Beleza',
  esportes: 'Esportes',
  livros: 'Livros',
  brinquedos: 'Brinquedos',
  infantil: 'Infantil',
  maternidade: 'Maternidade',
  alimentos: 'Alimentos',
  automotivo: 'Automotivo',
  pets: 'Pets',
};

/** Map from category key to i18n key */
export const CATEGORY_I18N_KEYS: Record<CategoryKey, string> = {
  ofertasGerais: 'config.categories.ofertasGerais',
  eletronicos: 'config.categories.eletronicos',
  eletrodomesticos: 'config.categories.eletrodomesticos',
  moda: 'config.categories.moda',
  homens: 'config.categories.homens',
  casaDecoracao: 'config.categories.casaDecoracao',
  beleza: 'config.categories.beleza',
  esportes: 'config.categories.esportes',
  livros: 'config.categories.livros',
  brinquedos: 'config.categories.brinquedos',
  infantil: 'config.categories.infantil',
  maternidade: 'config.categories.maternidade',
  alimentos: 'config.categories.alimentos',
  automotivo: 'config.categories.automotivo',
  pets: 'config.categories.pets',
};

/** Legacy categories array - uses display names for API compatibility */
export const CATEGORIES = Object.values(CATEGORY_DISPLAY_NAMES);

/** Category constants for code references */
export const ELETRONICOS_CATEGORY = 'Eletrônicos';
export const ELETRODOMESTICOS_CATEGORY = 'Eletrodomésticos';
export const MODA_CATEGORY = 'Moda';
export const HOMENS_CATEGORY = 'Homens';
export const ESPORTES_CATEGORY = 'Esportes';
export const CASA_CATEGORY = 'Casa e Decoração';
export const BELEZA_CATEGORY = 'Beleza';
export const GENERAL_CATEGORY = 'Ofertas gerais';

/** Automation timing constants */
export const MIN_AUTOMATION_INTERVAL_MINUTES = 5;
export const INTERVALS = [5, 10, 15, 20, 30, 45, 60, 90, 120];
export const MAX_SEND_GROUPS = 20;

/**
 * Kill-switch for the send_groups limits in the automation form:
 *  - global cap MAX_SEND_GROUPS (UI counter "0/20")
 *  - per-chip cap MAX_GROUPS_PER_INSTANCE (badges and toggle guard)
 *
 * Defaults to `false` so the UI lets users pick any number of groups, matching
 * the backend flag FEATURE_ENFORCE_GROUP_LIMITS (off by default). Flip to
 * `true` here AND set FEATURE_ENFORCE_GROUP_LIMITS=true on the server to
 * re-enable the WhatsApp-ban-protection cap end-to-end.
 */
export const ENFORCE_GROUP_LIMITS = false;

/** Electronics subcategories with labels and i18n keys */
export const ELETRONICOS_SUBCATEGORIES = {
  computadores: [
    {
      key: 'Eletrônicos - Computadores (Notebooks)',
      label: 'Notebooks',
      i18nKey: 'config.subcategories.eletronicos.computadores.notebooks',
    },
    {
      key: 'Eletrônicos - Computadores (SSD)',
      label: 'SSD',
      i18nKey: 'config.subcategories.eletronicos.computadores.ssd',
    },
    {
      key: 'Eletrônicos - Computadores (RAM)',
      label: 'Memória RAM',
      i18nKey: 'config.subcategories.eletronicos.computadores.ram',
    },
    {
      key: 'Eletrônicos - Computadores (Teclados)',
      label: 'Teclados',
      i18nKey: 'config.subcategories.eletronicos.computadores.teclados',
    },
    {
      key: 'Eletrônicos - Computadores (Monitores)',
      label: 'Monitores',
      i18nKey: 'config.subcategories.eletronicos.computadores.monitores',
    },
  ],
  celulares: [
    {
      key: 'Eletrônicos - Celulares (Smartphones)',
      label: 'Smartphones',
      i18nKey: 'config.subcategories.eletronicos.celulares.smartphones',
    },
    {
      key: 'Eletrônicos - Celulares (Capas)',
      label: 'Capas/Películas',
      i18nKey: 'config.subcategories.eletronicos.celulares.capas',
    },
    {
      key: 'Eletrônicos - Celulares (Carregadores)',
      label: 'Carregadores/Cabos',
      i18nKey: 'config.subcategories.eletronicos.celulares.carregadores',
    },
    {
      key: 'Eletrônicos - Celulares (Powerbank)',
      label: 'Powerbank',
      i18nKey: 'config.subcategories.eletronicos.celulares.powerbank',
    },
  ],
  audio: [
    {
      key: 'Eletrônicos - Áudio (Fones/Headsets)',
      label: 'Fones/Headsets',
      i18nKey: 'config.subcategories.eletronicos.audio.fonesHeadsets',
    },
    {
      key: 'Eletrônicos - Áudio (Caixas de Som)',
      label: 'Caixas de som',
      i18nKey: 'config.subcategories.eletronicos.audio.caixasDeSom',
    },
  ],
  games: [
    {
      key: 'Eletrônicos - Games (Consoles)',
      label: 'Consoles',
      i18nKey: 'config.subcategories.eletronicos.games.consoles',
    },
    {
      key: 'Eletrônicos - Games (Acessórios/Controles)',
      label: 'Controles/Acessórios',
      i18nKey: 'config.subcategories.eletronicos.games.acessoriosControles',
    },
  ],
  camerasDrones: [
    {
      key: 'Eletrônicos - Câmeras (Webcams)',
      label: 'Webcams',
      i18nKey: 'config.subcategories.eletronicos.camerasDrones.webcams',
    },
    {
      key: 'Eletrônicos - Drones',
      label: 'Drones',
      i18nKey: 'config.subcategories.eletronicos.camerasDrones.drones',
    },
  ],
  wearables: [
    {
      key: 'Eletrônicos - Wearables (Smartwatch)',
      label: 'Smartwatches',
      i18nKey: 'config.subcategories.eletronicos.wearables.smartwatch',
    },
  ],
} as const;

/** All electronics subcategory keys */
export const ALL_ELETRONICOS_SUBCATEGORY_KEYS = [
  ...ELETRONICOS_SUBCATEGORIES.computadores.map((s) => s.key),
  ...ELETRONICOS_SUBCATEGORIES.celulares.map((s) => s.key),
  ...ELETRONICOS_SUBCATEGORIES.audio.map((s) => s.key),
  ...ELETRONICOS_SUBCATEGORIES.games.map((s) => s.key),
  ...ELETRONICOS_SUBCATEGORIES.camerasDrones.map((s) => s.key),
  ...ELETRONICOS_SUBCATEGORIES.wearables.map((s) => s.key),
] as const;

/** Checks if value is an electronics subcategory key */
export const isEletronicosSubcategoryKey = (v: string): boolean =>
  (ALL_ELETRONICOS_SUBCATEGORY_KEYS as readonly string[]).includes(v);

/** Appliances subcategories with labels and i18n keys */
export const ELETRODOMESTICOS_SUBCATEGORIES = {
  cozinha: [
    {
      key: 'Eletrodomésticos - Cozinha (Airfryer)',
      label: 'Airfryer',
      i18nKey: 'config.subcategories.eletrodomesticos.cozinha.airfryer',
    },
    {
      key: 'Eletrodomésticos - Cozinha (Liquidificador/Espremedor)',
      label: 'Liquidificador/Espremedor',
      i18nKey: 'config.subcategories.eletrodomesticos.cozinha.liquidificadorEspremedor',
    },
    {
      key: 'Eletrodomésticos - Cozinha (Cafeteira)',
      label: 'Cafeteira',
      i18nKey: 'config.subcategories.eletrodomesticos.cozinha.cafeteira',
    },
    {
      key: 'Eletrodomésticos - Cozinha (Micro-ondas)',
      label: 'Micro-ondas',
      i18nKey: 'config.subcategories.eletrodomesticos.cozinha.microOndas',
    },
    {
      key: 'Eletrodomésticos - Cozinha (Torradeira/Sanduicheira)',
      label: 'Torradeira/Sanduicheira',
      i18nKey: 'config.subcategories.eletrodomesticos.cozinha.torradeiraSanduicheira',
    },
    {
      key: 'Eletrodomésticos - Cozinha (Batedeira/Mixer)',
      label: 'Batedeira/Mixer',
      i18nKey: 'config.subcategories.eletrodomesticos.cozinha.batedeiraMixer',
    },
    {
      key: 'Eletrodomésticos - Cozinha (Geladeira)',
      label: 'Geladeira',
      i18nKey: 'config.subcategories.eletrodomesticos.cozinha.geladeira',
    },
    {
      key: 'Eletrodomésticos - Cozinha (Filtro/Água)',
      label: 'Filtro/Purificador/Água',
      i18nKey: 'config.subcategories.eletrodomesticos.cozinha.filtroAgua',
    },
  ],
  climatizacao: [
    {
      key: 'Eletrodomésticos - Climatização (Ventilador)',
      label: 'Ventilador',
      i18nKey: 'config.subcategories.eletrodomesticos.climatizacao.ventilador',
    },
    {
      key: 'Eletrodomésticos - Climatização (Ar-condicionado)',
      label: 'Ar-condicionado',
      i18nKey: 'config.subcategories.eletrodomesticos.climatizacao.arCondicionado',
    },
    {
      key: 'Eletrodomésticos - Climatização (Portátil/Climatizador)',
      label: 'Portátil/Climatizador',
      i18nKey: 'config.subcategories.eletrodomesticos.climatizacao.portatilClimatizador',
    },
  ],
  lavanderia: [
    {
      key: 'Eletrodomésticos - Lavanderia (Lava e Seca)',
      label: 'Lava e seca',
      i18nKey: 'config.subcategories.eletrodomesticos.lavanderia.lavaESeca',
    },
    {
      key: 'Eletrodomésticos - Lavanderia (Ferro/Vaporizador)',
      label: 'Ferro/Vaporizador',
      i18nKey: 'config.subcategories.eletrodomesticos.lavanderia.ferroVaporizador',
    },
  ],
  limpeza: [
    {
      key: 'Eletrodomésticos - Limpeza (Aspirador/Robô)',
      label: 'Aspirador/Robô',
      i18nKey: 'config.subcategories.eletrodomesticos.limpeza.aspiradorRobo',
    },
  ],
  tvs: [
    {
      key: 'Eletrodomésticos - TVs (TV)',
      label: 'TVs',
      i18nKey: 'config.subcategories.eletrodomesticos.tvs.tv',
    },
  ],
} as const;

/** All appliances subcategory keys */
export const ALL_ELETRODOMESTICOS_SUBCATEGORY_KEYS = [
  ...ELETRODOMESTICOS_SUBCATEGORIES.cozinha.map((s) => s.key),
  ...ELETRODOMESTICOS_SUBCATEGORIES.climatizacao.map((s) => s.key),
  ...ELETRODOMESTICOS_SUBCATEGORIES.lavanderia.map((s) => s.key),
  ...ELETRODOMESTICOS_SUBCATEGORIES.limpeza.map((s) => s.key),
  ...ELETRODOMESTICOS_SUBCATEGORIES.tvs.map((s) => s.key),
] as const;

/** Checks if value is an appliances subcategory key */
export const isEletrodomesticosSubcategoryKey = (v: string): boolean =>
  (ALL_ELETRODOMESTICOS_SUBCATEGORY_KEYS as readonly string[]).includes(v);

/** Men's subcategories with labels and i18n keys */
export const HOMENS_SUBCATEGORIES = {
  roupas: [
    {
      key: 'Homens - Roupas (Camisetas)',
      label: 'Camisetas',
      i18nKey: 'config.subcategories.homens.roupas.camisetas',
    },
    {
      key: 'Homens - Roupas (Bermudas)',
      label: 'Bermudas/Shorts',
      i18nKey: 'config.subcategories.homens.roupas.bermudas',
    },
    {
      key: 'Homens - Roupas (Jeans)',
      label: 'Jeans',
      i18nKey: 'config.subcategories.homens.roupas.jeans',
    },
    {
      key: 'Homens - Roupas (Calças Cargo)',
      label: 'Calças (Cargo)',
      i18nKey: 'config.subcategories.homens.roupas.calcasCargo',
    },
    {
      key: 'Homens - Roupas (Moletons)',
      label: 'Moletons',
      i18nKey: 'config.subcategories.homens.roupas.moletons',
    },
    {
      key: 'Homens - Roupas (Jaquetas)',
      label: 'Jaquetas',
      i18nKey: 'config.subcategories.homens.roupas.jaquetas',
    },
  ],
  acessorios: [
    {
      key: 'Homens - Acessórios (Bonés)',
      label: 'Bonés',
      i18nKey: 'config.subcategories.homens.acessorios.bones',
    },
    {
      key: 'Homens - Acessórios (Relógios)',
      label: 'Relógios',
      i18nKey: 'config.subcategories.homens.acessorios.relogios',
    },
    {
      key: 'Homens - Acessórios (Carteiras/Porta-cartões)',
      label: 'Carteiras/Porta-cartões',
      i18nKey: 'config.subcategories.homens.acessorios.carteirasPortaCartoes',
    },
    {
      key: 'Homens - Acessórios (Cintos)',
      label: 'Cintos',
      i18nKey: 'config.subcategories.homens.acessorios.cintos',
    },
    {
      key: 'Homens - Acessórios (Óculos de sol)',
      label: 'Óculos de sol',
      i18nKey: 'config.subcategories.homens.acessorios.oculosDeSol',
    },
  ],
  calcados: [
    {
      key: 'Homens - Calçados (Tênis)',
      label: 'Tênis',
      i18nKey: 'config.subcategories.homens.calcados.tenis',
    },
    {
      key: 'Homens - Calçados (Chinelos)',
      label: 'Chinelos',
      i18nKey: 'config.subcategories.homens.calcados.chinelos',
    },
    {
      key: 'Homens - Calçados (Botas)',
      label: 'Botas',
      i18nKey: 'config.subcategories.homens.calcados.botas',
    },
    {
      key: 'Homens - Calçados (Sociais/Oxford)',
      label: 'Sociais/Oxford',
      i18nKey: 'config.subcategories.homens.calcados.sociaisOxford',
    },
  ],
  intimas: [
    {
      key: 'Homens - Íntimas (Cuecas)',
      label: 'Cuecas',
      i18nKey: 'config.subcategories.homens.intimas.cuecas',
    },
    {
      key: 'Homens - Íntimas (Meias)',
      label: 'Meias',
      i18nKey: 'config.subcategories.homens.intimas.meias',
    },
  ],
  barbearia: [
    {
      key: 'Homens - Barbearia (Aparadores)',
      label: 'Aparadores/Máquinas',
      i18nKey: 'config.subcategories.homens.barbearia.aparadores',
    },
    {
      key: 'Homens - Barbearia (Cremes/Géis)',
      label: 'Cremes/Espumas/Géis',
      i18nKey: 'config.subcategories.homens.barbearia.cremesGeis',
    },
  ],
  perfumes: [
    {
      key: 'Homens - Perfumes',
      label: 'Perfumes',
      i18nKey: 'config.subcategories.homens.perfumes',
    },
  ],
} as const;

/** All men's subcategory keys */
export const ALL_HOMENS_SUBCATEGORY_KEYS = [
  ...HOMENS_SUBCATEGORIES.roupas.map((s) => s.key),
  ...HOMENS_SUBCATEGORIES.acessorios.map((s) => s.key),
  ...HOMENS_SUBCATEGORIES.calcados.map((s) => s.key),
  ...HOMENS_SUBCATEGORIES.intimas.map((s) => s.key),
  ...HOMENS_SUBCATEGORIES.barbearia.map((s) => s.key),
  ...HOMENS_SUBCATEGORIES.perfumes.map((s) => s.key),
] as const;

/** Checks if value is a men's subcategory key */
export const isHomensSubcategoryKey = (v: string): boolean =>
  (ALL_HOMENS_SUBCATEGORY_KEYS as readonly string[]).includes(v);

/** Sports subcategories with labels and i18n keys */
export const ESPORTES_SUBCATEGORIES = {
  fitness: [
    {
      key: 'Esportes - Fitness (Camisetas)',
      label: 'Camisetas',
      i18nKey: 'config.subcategories.esportes.fitness.camisetas',
    },
    {
      key: 'Esportes - Fitness (Regatas)',
      label: 'Regatas',
      i18nKey: 'config.subcategories.esportes.fitness.regatas',
    },
    {
      key: 'Esportes - Fitness (Musculação/Acessórios)',
      label: 'Musculação/Acessórios',
      i18nKey: 'config.subcategories.esportes.fitness.musculacaoAcessorios',
    },
    {
      key: 'Esportes - Fitness (Máquinas)',
      label: 'Máquinas (esteira/bike/etc)',
      i18nKey: 'config.subcategories.esportes.fitness.maquinas',
    },
  ],
  outdoor: [
    {
      key: 'Esportes - Outdoor/Ciclismo (Acessórios)',
      label: 'Outdoor/Ciclismo (Acessórios)',
      i18nKey: 'config.subcategories.esportes.outdoor.ciclismoAcessorios',
    },
  ],
  futebol: [
    {
      key: 'Esportes - Futebol (Bolas)',
      label: 'Futebol: Bolas',
      i18nKey: 'config.subcategories.esportes.futebol.bolas',
    },
  ],
  lutas: [
    {
      key: 'Esportes - Lutas (Luvas/Bandagens)',
      label: 'Lutas: Luvas/Bandagens',
      i18nKey: 'config.subcategories.esportes.lutas.luvasBandagens',
    },
    {
      key: 'Esportes - Lutas (Sacos de Pancada)',
      label: 'Lutas: Sacos de pancada',
      i18nKey: 'config.subcategories.esportes.lutas.sacosDePancada',
    },
  ],
} as const;

/** All sports subcategory keys */
export const ALL_ESPORTES_SUBCATEGORY_KEYS = [
  ...ESPORTES_SUBCATEGORIES.fitness.map((s) => s.key),
  ...ESPORTES_SUBCATEGORIES.outdoor.map((s) => s.key),
  ...ESPORTES_SUBCATEGORIES.futebol.map((s) => s.key),
  ...ESPORTES_SUBCATEGORIES.lutas.map((s) => s.key),
] as const;

/** Checks if value is a sports subcategory key */
export const isEsportesSubcategoryKey = (v: string): boolean =>
  (ALL_ESPORTES_SUBCATEGORY_KEYS as readonly string[]).includes(v);

/** Fashion subcategories with labels and i18n keys */
export const MODA_SUBCATEGORIES = {
  feminina: [
    {
      key: 'Moda Feminina - Vestidos',
      label: 'Vestidos',
      i18nKey: 'config.subcategories.moda.feminina.vestidos',
    },
    {
      key: 'Moda Feminina - Shorts',
      label: 'Shorts',
      i18nKey: 'config.subcategories.moda.feminina.shorts',
    },
    {
      key: 'Moda Feminina - Calçados (Tênis)',
      label: 'Calçados: Tênis',
      i18nKey: 'config.subcategories.moda.feminina.calcadosTenis',
    },
    {
      key: 'Moda Feminina - Calçados (Plataformas)',
      label: 'Calçados: Plataformas',
      i18nKey: 'config.subcategories.moda.feminina.calcadosPlataformas',
    },
    {
      key: 'Moda Feminina - Calças',
      label: 'Calças',
      i18nKey: 'config.subcategories.moda.feminina.calcas',
    },
    {
      key: 'Moda Feminina - Jeans',
      label: 'Jeans',
      i18nKey: 'config.subcategories.moda.feminina.jeans',
    },
    {
      key: 'Moda Feminina - Camisetas',
      label: 'Camisetas',
      i18nKey: 'config.subcategories.moda.feminina.camisetas',
    },
    {
      key: 'Moda Feminina - Blusas',
      label: 'Blusas',
      i18nKey: 'config.subcategories.moda.feminina.blusas',
    },
    {
      key: 'Moda Feminina - Bolsas',
      label: 'Bolsas',
      i18nKey: 'config.subcategories.moda.feminina.bolsas',
    },
    {
      key: 'Moda Feminina - Lingerie (Calcinhas)',
      label: 'Lingerie: Calcinhas',
      i18nKey: 'config.subcategories.moda.feminina.lingerieCalcinhas',
    },
  ],
  masculina: [
    {
      key: 'Moda Masculina - Bermudas',
      label: 'Bermudas',
      i18nKey: 'config.subcategories.moda.masculina.bermudas',
    },
    {
      key: 'Moda Masculina - Camisas',
      label: 'Camisas',
      i18nKey: 'config.subcategories.moda.masculina.camisas',
    },
    {
      key: 'Moda Masculina - Jeans',
      label: 'Jeans',
      i18nKey: 'config.subcategories.moda.masculina.jeans',
    },
    {
      key: 'Moda Masculina - Calçados (Tênis)',
      label: 'Calçados: Tênis',
      i18nKey: 'config.subcategories.moda.masculina.calcadosTenis',
    },
    {
      key: 'Moda Masculina - Calçados (Chinelos)',
      label: 'Calçados: Chinelos',
      i18nKey: 'config.subcategories.moda.masculina.calcadosChinelos',
    },
  ],
} as const;

/** All fashion subcategory keys */
export const ALL_MODA_SUBCATEGORY_KEYS = [
  ...MODA_SUBCATEGORIES.feminina.map((s) => s.key),
  ...MODA_SUBCATEGORIES.masculina.map((s) => s.key),
] as const;

/** Checks if value is a fashion subcategory key */
export const isModaSubcategoryKey = (v: string): boolean =>
  (ALL_MODA_SUBCATEGORY_KEYS as readonly string[]).includes(v);

/** Home & decor subcategories with labels and i18n keys */
export const CASA_SUBCATEGORIES = {
  cozinha: [
    {
      key: 'Casa - Cozinha (Armazenamento)',
      label: 'Cozinha: Armazenamento (potes/marmitas)',
      i18nKey: 'config.subcategories.casa.cozinha.armazenamento',
    },
    {
      key: 'Casa - Cozinha (Cortadores)',
      label: 'Cozinha: Cortadores/Raladores',
      i18nKey: 'config.subcategories.casa.cozinha.cortadores',
    },
  ],
  quarto: [
    {
      key: 'Casa - Quarto (Cama/Edredom)',
      label: 'Quarto: Cama/Edredom/Colchas',
      i18nKey: 'config.subcategories.casa.quarto.camaEdredom',
    },
  ],
  decoracao: [
    {
      key: 'Casa - Decoração (Tapetes)',
      label: 'Decoração: Tapetes',
      i18nKey: 'config.subcategories.casa.decoracao.tapetes',
    },
    {
      key: 'Casa - Decoração (Cortinas)',
      label: 'Decoração: Cortinas/Persianas',
      i18nKey: 'config.subcategories.casa.decoracao.cortinas',
    },
    {
      key: 'Casa - Decoração (Vasos)',
      label: 'Decoração: Vasos',
      i18nKey: 'config.subcategories.casa.decoracao.vasos',
    },
    {
      key: 'Casa - Decoração (Parede/Molduras)',
      label: 'Decoração: Parede/Molduras',
      i18nKey: 'config.subcategories.casa.decoracao.paredeMolduras',
    },
  ],
  banheiro: [
    {
      key: 'Casa - Banheiro (Suportes/Sabonete)',
      label: 'Banheiro: Suportes/Saboneteiras',
      i18nKey: 'config.subcategories.casa.banheiro.suportesSabonete',
    },
  ],
  organizacao: [
    {
      key: 'Casa - Organização (Guarda-roupa)',
      label: 'Organização: Guarda-roupa',
      i18nKey: 'config.subcategories.casa.organizacao.guardaRoupa',
    },
    {
      key: 'Casa - Organização (Sapateiras)',
      label: 'Organização: Sapateiras',
      i18nKey: 'config.subcategories.casa.organizacao.sapateiras',
    },
  ],
  moveis: [
    {
      key: 'Casa - Móveis (Guarda-roupas)',
      label: 'Móveis: Guarda-roupas',
      i18nKey: 'config.subcategories.casa.moveis.guardaRoupas',
    },
    {
      key: 'Casa - Móveis (Armários/Gabinetes)',
      label: 'Móveis: Armários/Gabinetes',
      i18nKey: 'config.subcategories.casa.moveis.armariosGabinetes',
    },
    {
      key: 'Casa - Móveis (Camas/Cabeceiras)',
      label: 'Móveis: Camas/Cabeceiras',
      i18nKey: 'config.subcategories.casa.moveis.camasCabeceiras',
    },
    {
      key: 'Casa - Móveis (Sofás)',
      label: 'Móveis: Sofás',
      i18nKey: 'config.subcategories.casa.moveis.sofas',
    },
    {
      key: 'Casa - Móveis (Cadeiras/Banquetas)',
      label: 'Móveis: Cadeiras/Banquetas',
      i18nKey: 'config.subcategories.casa.moveis.cadeirasBanquetas',
    },
    {
      key: 'Casa - Móveis (Mesas/Escrivaninhas)',
      label: 'Móveis: Mesas/Escrivaninhas',
      i18nKey: 'config.subcategories.casa.moveis.mesasEscrivaninhas',
    },
    {
      key: 'Casa - Móveis (Prateleiras/Racks)',
      label: 'Móveis: Prateleiras/Racks',
      i18nKey: 'config.subcategories.casa.moveis.prateleirasRacks',
    },
  ],
} as const;

/** All home & decor subcategory keys */
export const ALL_CASA_SUBCATEGORY_KEYS = [
  ...CASA_SUBCATEGORIES.cozinha.map((s) => s.key),
  ...CASA_SUBCATEGORIES.quarto.map((s) => s.key),
  ...CASA_SUBCATEGORIES.decoracao.map((s) => s.key),
  ...CASA_SUBCATEGORIES.banheiro.map((s) => s.key),
  ...CASA_SUBCATEGORIES.organizacao.map((s) => s.key),
  ...CASA_SUBCATEGORIES.moveis.map((s) => s.key),
] as const;

/** Checks if value is a home & decor subcategory key */
export const isCasaSubcategoryKey = (v: string): boolean =>
  (ALL_CASA_SUBCATEGORY_KEYS as readonly string[]).includes(v);

/** Beauty subcategories with labels and i18n keys */
export const BELEZA_SUBCATEGORIES = {
  perfumes: [
    {
      key: 'Beleza - Perfumes e Fragrâncias',
      label: 'Perfumes e Fragrâncias',
      i18nKey: 'config.subcategories.beleza.perfumes',
    },
  ],
  skincare: [
    {
      key: 'Beleza - Skincare (Hidratante Facial)',
      label: 'Skincare: Hidratante facial',
      i18nKey: 'config.subcategories.beleza.skincare.hidratanteFacial',
    },
    {
      key: 'Beleza - Skincare (Protetor Solar Facial)',
      label: 'Skincare: Protetor solar facial',
      i18nKey: 'config.subcategories.beleza.skincare.protetorSolarFacial',
    },
    {
      key: 'Beleza - Skincare (Soro/Essência)',
      label: 'Skincare: Soro/Essência',
      i18nKey: 'config.subcategories.beleza.skincare.soroEssencia',
    },
    {
      key: 'Beleza - Skincare (Máscaras)',
      label: 'Skincare: Máscaras',
      i18nKey: 'config.subcategories.beleza.skincare.mascaras',
    },
    {
      key: 'Beleza - Skincare (Massagem Facial)',
      label: 'Skincare: Massagem facial',
      i18nKey: 'config.subcategories.beleza.skincare.massagemFacial',
    },
  ],
  maquiagem: [
    {
      key: 'Beleza - Maquiagem (Base)',
      label: 'Maquiagem: Base',
      i18nKey: 'config.subcategories.beleza.maquiagem.base',
    },
    {
      key: 'Beleza - Maquiagem (Corretivo)',
      label: 'Maquiagem: Corretivo',
      i18nKey: 'config.subcategories.beleza.maquiagem.corretivo',
    },
    {
      key: 'Beleza - Maquiagem (Pó)',
      label: 'Maquiagem: Pó',
      i18nKey: 'config.subcategories.beleza.maquiagem.po',
    },
    {
      key: 'Beleza - Maquiagem (Kits)',
      label: 'Maquiagem: Kits',
      i18nKey: 'config.subcategories.beleza.maquiagem.kits',
    },
  ],
  cabelo: [
    {
      key: 'Beleza - Cabelo (Shampoo)',
      label: 'Cabelo: Shampoo',
      i18nKey: 'config.subcategories.beleza.cabelo.shampoo',
    },
    {
      key: 'Beleza - Cabelo (Tratamento)',
      label: 'Cabelo: Tratamento',
      i18nKey: 'config.subcategories.beleza.cabelo.tratamento',
    },
    {
      key: 'Beleza - Cabelo (Finalizadores)',
      label: 'Cabelo: Finalizadores',
      i18nKey: 'config.subcategories.beleza.cabelo.finalizadores',
    },
    {
      key: 'Beleza - Cabelo (Tintura)',
      label: 'Cabelo: Tintura',
      i18nKey: 'config.subcategories.beleza.cabelo.tintura',
    },
  ],
  corpo: [
    {
      key: 'Beleza - Corpo (Desodorantes/Body Splash)',
      label: 'Corpo: Desodorantes/Body splash',
      i18nKey: 'config.subcategories.beleza.corpo.desodorantesBodySplash',
    },
    {
      key: 'Beleza - Corpo (Hidratante Corporal)',
      label: 'Corpo: Hidratante corporal',
      i18nKey: 'config.subcategories.beleza.corpo.hidratanteCorporal',
    },
    {
      key: 'Beleza - Corpo (Sabonetes)',
      label: 'Corpo: Sabonetes',
      i18nKey: 'config.subcategories.beleza.corpo.sabonetes',
    },
    {
      key: 'Beleza - Corpo (Depilação)',
      label: 'Corpo: Depilação',
      i18nKey: 'config.subcategories.beleza.corpo.depilacao',
    },
  ],
  unhas: [
    {
      key: 'Beleza - Unhas (Gel)',
      label: 'Unhas: Gel',
      i18nKey: 'config.subcategories.beleza.unhas.gel',
    },
    {
      key: 'Beleza - Unhas (Ferramentas)',
      label: 'Unhas: Ferramentas',
      i18nKey: 'config.subcategories.beleza.unhas.ferramentas',
    },
  ],
} as const;

/** All beauty subcategory keys */
export const ALL_BELEZA_SUBCATEGORY_KEYS = [
  ...BELEZA_SUBCATEGORIES.perfumes.map((s) => s.key),
  ...BELEZA_SUBCATEGORIES.skincare.map((s) => s.key),
  ...BELEZA_SUBCATEGORIES.maquiagem.map((s) => s.key),
  ...BELEZA_SUBCATEGORIES.cabelo.map((s) => s.key),
  ...BELEZA_SUBCATEGORIES.corpo.map((s) => s.key),
  ...BELEZA_SUBCATEGORIES.unhas.map((s) => s.key),
] as const;

/** Checks if value is a beauty subcategory key */
export const isBelezaSubcategoryKey = (v: string): boolean =>
  (ALL_BELEZA_SUBCATEGORY_KEYS as readonly string[]).includes(v);

/** Subcategory groups for use with CategoryRefinementSection component */
export const ELETRONICOS_GROUPS: SubcategoryGroup[] = [
  {
    name: 'Computadores',
    i18nKey: 'config.groups.eletronicos.computadores',
    items: ELETRONICOS_SUBCATEGORIES.computadores,
  },
  {
    name: 'Celulares',
    i18nKey: 'config.groups.eletronicos.celulares',
    items: ELETRONICOS_SUBCATEGORIES.celulares,
  },
  {
    name: 'Áudio',
    i18nKey: 'config.groups.eletronicos.audio',
    items: ELETRONICOS_SUBCATEGORIES.audio,
  },
  {
    name: 'Games',
    i18nKey: 'config.groups.eletronicos.games',
    items: ELETRONICOS_SUBCATEGORIES.games,
  },
  {
    name: 'Câmeras & Drones',
    i18nKey: 'config.groups.eletronicos.camerasDrones',
    items: ELETRONICOS_SUBCATEGORIES.camerasDrones,
  },
  {
    name: 'Wearables',
    i18nKey: 'config.groups.eletronicos.wearables',
    items: ELETRONICOS_SUBCATEGORIES.wearables,
  },
];

export const ELETRODOMESTICOS_GROUPS: SubcategoryGroup[] = [
  {
    name: 'Cozinha',
    i18nKey: 'config.groups.eletrodomesticos.cozinha',
    items: ELETRODOMESTICOS_SUBCATEGORIES.cozinha,
  },
  {
    name: 'Climatização',
    i18nKey: 'config.groups.eletrodomesticos.climatizacao',
    items: ELETRODOMESTICOS_SUBCATEGORIES.climatizacao,
  },
  {
    name: 'Lavanderia',
    i18nKey: 'config.groups.eletrodomesticos.lavanderia',
    items: ELETRODOMESTICOS_SUBCATEGORIES.lavanderia,
  },
  {
    name: 'Limpeza',
    i18nKey: 'config.groups.eletrodomesticos.limpeza',
    items: ELETRODOMESTICOS_SUBCATEGORIES.limpeza,
  },
  {
    name: 'TVs',
    i18nKey: 'config.groups.eletrodomesticos.tvs',
    items: ELETRODOMESTICOS_SUBCATEGORIES.tvs,
  },
];

export const HOMENS_GROUPS: SubcategoryGroup[] = [
  { name: 'Roupas', i18nKey: 'config.groups.homens.roupas', items: HOMENS_SUBCATEGORIES.roupas },
  {
    name: 'Acessórios',
    i18nKey: 'config.groups.homens.acessorios',
    items: HOMENS_SUBCATEGORIES.acessorios,
  },
  {
    name: 'Calçados',
    i18nKey: 'config.groups.homens.calcados',
    items: HOMENS_SUBCATEGORIES.calcados,
  },
  { name: 'Íntimas', i18nKey: 'config.groups.homens.intimas', items: HOMENS_SUBCATEGORIES.intimas },
  {
    name: 'Barbearia',
    i18nKey: 'config.groups.homens.barbearia',
    items: HOMENS_SUBCATEGORIES.barbearia,
  },
  {
    name: 'Perfumes',
    i18nKey: 'config.groups.homens.perfumes',
    items: HOMENS_SUBCATEGORIES.perfumes,
  },
];

export const ESPORTES_GROUPS: SubcategoryGroup[] = [
  {
    name: 'Fitness',
    i18nKey: 'config.groups.esportes.fitness',
    items: ESPORTES_SUBCATEGORIES.fitness,
  },
  {
    name: 'Outdoor/Ciclismo',
    i18nKey: 'config.groups.esportes.outdoor',
    items: ESPORTES_SUBCATEGORIES.outdoor,
  },
  {
    name: 'Futebol',
    i18nKey: 'config.groups.esportes.futebol',
    items: ESPORTES_SUBCATEGORIES.futebol,
  },
  { name: 'Lutas', i18nKey: 'config.groups.esportes.lutas', items: ESPORTES_SUBCATEGORIES.lutas },
];

export const MODA_GROUPS: SubcategoryGroup[] = [
  { name: 'Feminina', i18nKey: 'config.groups.moda.feminina', items: MODA_SUBCATEGORIES.feminina },
  {
    name: 'Masculina',
    i18nKey: 'config.groups.moda.masculina',
    items: MODA_SUBCATEGORIES.masculina,
  },
];

export const CASA_GROUPS: SubcategoryGroup[] = [
  { name: 'Móveis', i18nKey: 'config.groups.casa.moveis', items: CASA_SUBCATEGORIES.moveis },
  {
    name: 'Decoração',
    i18nKey: 'config.groups.casa.decoracao',
    items: CASA_SUBCATEGORIES.decoracao,
  },
  {
    name: 'Organização',
    i18nKey: 'config.groups.casa.organizacao',
    items: CASA_SUBCATEGORIES.organizacao,
  },
  { name: 'Cozinha', i18nKey: 'config.groups.casa.cozinha', items: CASA_SUBCATEGORIES.cozinha },
  { name: 'Quarto', i18nKey: 'config.groups.casa.quarto', items: CASA_SUBCATEGORIES.quarto },
  { name: 'Banheiro', i18nKey: 'config.groups.casa.banheiro', items: CASA_SUBCATEGORIES.banheiro },
];

export const BELEZA_GROUPS: SubcategoryGroup[] = [
  {
    name: 'Skincare',
    i18nKey: 'config.groups.beleza.skincare',
    items: BELEZA_SUBCATEGORIES.skincare,
  },
  { name: 'Cabelo', i18nKey: 'config.groups.beleza.cabelo', items: BELEZA_SUBCATEGORIES.cabelo },
  {
    name: 'Maquiagem',
    i18nKey: 'config.groups.beleza.maquiagem',
    items: BELEZA_SUBCATEGORIES.maquiagem,
  },
  { name: 'Corpo', i18nKey: 'config.groups.beleza.corpo', items: BELEZA_SUBCATEGORIES.corpo },
  { name: 'Unhas', i18nKey: 'config.groups.beleza.unhas', items: BELEZA_SUBCATEGORIES.unhas },
  {
    name: 'Perfumes',
    i18nKey: 'config.groups.beleza.perfumes',
    items: BELEZA_SUBCATEGORIES.perfumes,
  },
];

/**
 * Clamps interval minutes to minimum allowed value.
 * @param v - Value to clamp (converted to number)
 * @returns Clamped interval in minutes
 */
export const clampIntervalMinutes = (v: unknown): number => {
  const n = Number(v);
  if (!Number.isFinite(n)) return MIN_AUTOMATION_INTERVAL_MINUTES;
  return Math.max(MIN_AUTOMATION_INTERVAL_MINUTES, Math.floor(n));
};

/**
 * Transforms textarea text into array of strings.
 * @param value - Raw textarea content with newlines
 * @returns Array of trimmed, non-empty strings
 */
export const parseRulesTextarea = (value: string): string[] =>
  value
    .split('\n')
    .map((s) => s.trim())
    .filter(Boolean);

/**
 * Error code patterns for Shopee affiliate API.
 * Multiple keywords per error type for robustness against API message changes.
 * Codes are primary identifiers; keywords are fallback detection.
 */
const SHOPEE_ERROR_PATTERNS = {
  noAccess: {
    codes: ['10035'],
    keywords: [
      'do not have access to the shopee affiliate open api',
      'no access',
      'access denied',
      'unauthorized access',
    ],
  },
  rateLimit: {
    codes: ['10030'],
    keywords: ['rate limit', 'too many requests', 'throttle', 'quota exceeded'],
  },
  invalidSignature: {
    codes: ['10020'],
    keywords: ['invalid signature', 'signature mismatch', 'bad signature'],
  },
  authNotFound: {
    codes: ['10010'],
    keywords: [
      'affiliate authorization not found',
      'authorization not found',
      'auth not found',
      'not authorized',
    ],
  },
  invalidToken: {
    codes: ['10015'],
    keywords: ['invalid token', 'token expired', 'token invalid'],
  },
} as const;

/**
 * Translates Shopee affiliate API errors to user-friendly i18n keys.
 * Uses error codes as primary identifier, keywords as fallback.
 * @param rawMessage - Raw error message from API
 * @param t - i18n translation function
 * @returns Translated error message or original if no match
 */
export const translateShopeeAffiliateError = (
  rawMessage: string,
  t: (key: string) => string
): string => {
  const msg = String(rawMessage || '').trim();
  if (!msg) return msg;

  const codeMatch = msg.match(/\[(\d+)\]/) || msg.match(/\bcode\b[:\s]+(\d+)\b/i);
  const code = codeMatch?.[1] ?? null;
  const lowerMsg = msg.toLowerCase();

  const matchesPattern = (pattern: { codes: readonly string[]; keywords: readonly string[] }) =>
    pattern.codes.includes(code ?? '') || pattern.keywords.some((k) => lowerMsg.includes(k));

  if (matchesPattern(SHOPEE_ERROR_PATTERNS.noAccess)) {
    return t('config.errors.shopeeAffiliate.noAccess');
  }

  if (matchesPattern(SHOPEE_ERROR_PATTERNS.rateLimit)) {
    return t('config.errors.shopeeAffiliate.rateLimit');
  }

  if (matchesPattern(SHOPEE_ERROR_PATTERNS.invalidSignature)) {
    return t('config.errors.shopeeAffiliate.invalidSignature');
  }

  if (matchesPattern(SHOPEE_ERROR_PATTERNS.authNotFound)) {
    return t('config.errors.shopeeAffiliate.authNotFound');
  }

  if (matchesPattern(SHOPEE_ERROR_PATTERNS.invalidToken)) {
    return t('config.errors.shopeeAffiliate.invalidToken');
  }

  return msg;
};
