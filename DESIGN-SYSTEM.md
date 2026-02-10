# Design System - Automatiza

> Sistema de design para garantir consistência visual em todas as páginas do projeto.

## Stack Tecnológica

- **CSS Framework:** Tailwind CSS v4.1
- **Componentes:** shadcn/ui
- **Gráficos:** Recharts
- **Ícones:** Lucide React

---

## 1. Paleta de Cores

### 1.1 Cores Primárias

| Nome | HEX | HSL | Uso |
|------|-----|-----|-----|
| Wine (Primary) | `#8D2D53` | `336 78% 28%` | Cor principal, ícones, textos de destaque |
| Orange (Secondary) | `#F5820D` | `34 94% 51%` | Acentos, CTAs secundários |

### 1.2 Cores Semânticas

| Nome | HEX | HSL | Uso |
|------|-----|-----|-----|
| Success | `#22C55E` | `142 76% 36%` | Estados de sucesso |
| Destructive | `#EF4444` | `0 84% 60%` | Erros, ações destrutivas |
| Muted | `#E5E7EB` | `220 13% 91%` | Backgrounds sutis |

### 1.3 Paleta Wine Expandida (Dashboard)

```typescript
const colors = {
  wine: '#8D2D53',
  wineBg: 'rgba(141, 45, 83, 0.08)',
  wineBgHover: 'rgba(141, 45, 83, 0.12)',
  wineBorder: 'rgba(141, 45, 83, 0.2)',
  wineText: '#8D2D53',
  wineTextLight: 'rgba(141, 45, 83, 0.7)',
  wineTextMuted: 'rgba(141, 45, 83, 0.6)',
  iconBg: 'rgba(141, 45, 83, 0.1)',
  iconBgHover: 'rgba(141, 45, 83, 0.15)',
};
```

---

## 2. Tipografia

### 2.1 Font Family

- **Família:** `Inter`, fallback para system fonts
- **Definição:** `--font-sans` em `index.css`

### 2.2 Font Sizes

| Token | Tamanho | Line Height | Weight | Uso |
|-------|---------|-------------|--------|-----|
| title | 24px | 1.3 | 600 | Títulos principais |
| subtitle | 18px | 1.4 | 600 | Subtítulos |
| base | 14px | 1.5 | 400 | Texto padrão |
| label | 12px | 1.5 | 500 | Labels, legendas |

### 2.3 Padrões Responsivos de Texto

```html
<!-- Títulos -->
text-xl xs:text-2xl md:text-3xl font-semibold tracking-tight

<!-- Labels uppercase -->
uppercase text-[10px] xs:text-xs tracking-[0.1em] xs:tracking-[0.12em]

<!-- Texto descritivo -->
text-xs xs:text-sm tracking-tight
```

---

## 3. Sistema de Espaçamento

### 3.1 Border Radius

| Token | Valor | Uso |
|-------|-------|-----|
| base | 0.75rem (12px) | Padrão geral |
| lg | var(--radius) | Cards grandes |
| md | calc(var(--radius) - 2px) | Elementos médios |
| sm | calc(var(--radius) - 4px) | Botões, badges |

### 3.2 Padding Responsivo

```html
<!-- Container principal -->
p-4 xs:p-5 sm:p-6 md:p-8

<!-- Cards menores -->
p-3 xs:p-4

<!-- Icon containers -->
p-1.5 xs:p-2
```

### 3.3 Gap Responsivo

```html
<!-- Grids -->
gap-3 xs:gap-4 sm:gap-6 md:gap-8

<!-- Flex items -->
gap-2 xs:gap-3

<!-- Inline items -->
gap-1.5 xs:gap-2
```

---

## 4. Sistema Responsivo

### 4.1 Breakpoints

| Nome | Valor | Uso |
|------|-------|-----|
| xs | 480px | Mobile landscape |
| sm | 640px | Tablet pequeno |
| md | 768px | Tablet |
| lg | 1024px | Desktop pequeno |
| xl | 1280px | Desktop |
| 2xl | 1536px | Desktop grande |

### 4.2 Padrão Mobile-First

```html
<!-- Sempre do menor para o maior -->
class="base xs:larger sm:even-larger md:desktop"
```

---

## 5. Componentes do Dashboard

### 5.1 Container Principal

```html
<div
  className="flex flex-col w-full border rounded-xl xs:rounded-2xl p-4 xs:p-5 sm:p-6 md:p-8"
  style={{ backgroundColor: '#ffffff', borderColor: 'rgba(228, 228, 231, 0.6)' }}
>
```

### 5.2 Stat Cards

```html
<div
  className="rounded-xl xs:rounded-2xl border p-4 xs:p-5 sm:p-6 transition-all cursor-default"
  style={{ backgroundColor: colors.wineBg, borderColor: colors.wineBorder }}
  onMouseEnter={(e) => e.currentTarget.style.backgroundColor = colors.wineBgHover}
  onMouseLeave={(e) => e.currentTarget.style.backgroundColor = colors.wineBg}
>
```

### 5.3 Icon Containers

```html
<div
  className="p-1.5 xs:p-2 rounded-lg xs:rounded-xl border"
  style={{ backgroundColor: colors.iconBg, borderColor: colors.wineBorder }}
>
  <Icon className="w-5 h-5 xs:w-6 xs:h-6" style={{ color: colors.wine }} />
</div>
```

### 5.4 Header com Ícone

```html
<div className="flex items-center gap-2 xs:gap-3">
  <div className="p-1.5 xs:p-2 rounded-lg" style={{ backgroundColor: colors.iconBg }}>
    <Icon className="w-4 h-4 xs:w-5 xs:h-5" style={{ color: colors.wine }} />
  </div>
  <span className="uppercase text-[10px] xs:text-xs tracking-widest" style={{ color: colors.wineTextLight }}>
    {label}
  </span>
</div>
```

---

## 6. Gráficos (Recharts)

### 6.1 Configuração Base

```tsx
<ResponsiveContainer width="100%" height="100%">
  <AreaChart data={chartData} margin={{ top: 10, right: 10, left: 0, bottom: 0 }}>
    <defs>
      <linearGradient id="colorMembers" x1="0" y1="0" x2="0" y2="1">
        <stop offset="5%" stopColor="#8D2D53" stopOpacity={0.3} />
        <stop offset="95%" stopColor="#8D2D53" stopOpacity={0} />
      </linearGradient>
    </defs>
    <CartesianGrid strokeDasharray="3 3" stroke="rgba(141, 45, 83, 0.1)" vertical={false} />
    <XAxis
      dataKey="label"
      axisLine={false}
      tickLine={false}
      tick={{ fill: 'rgba(141, 45, 83, 0.6)', fontSize: 12 }}
    />
    <Tooltip
      contentStyle={{
        backgroundColor: '#ffffff',
        border: '1px solid rgba(141, 45, 83, 0.2)',
        borderRadius: '12px'
      }}
    />
    <Area
      type="monotone"
      dataKey="members"
      stroke="#8D2D53"
      strokeWidth={2.5}
      fillOpacity={1}
      fill="url(#colorMembers)"
    />
  </AreaChart>
</ResponsiveContainer>
```

---

## 7. Botões (shadcn/ui)

### 7.1 Variantes

| Variante | Uso | Classes |
|----------|-----|---------|
| default | Ação primária | `bg-primary text-primary-foreground` |
| outline | Ação secundária | `border border-input hover:bg-[#8D2D53] hover:text-white` |
| ghost | Ação terciária | `hover:bg-[#8D2D53] hover:text-white` |
| destructive | Ação perigosa | `bg-destructive text-destructive-foreground` |

### 7.2 Tamanhos

| Tamanho | Classes |
|---------|---------|
| default | `h-10 px-4 py-2` |
| sm | `h-9 rounded-md px-3` |
| lg | `h-11 rounded-md px-8` |
| icon | `h-10 w-10` |

---

## 8. Badges

```html
<!-- Success -->
<Badge variant="success">Ativo</Badge>

<!-- Default (Wine) -->
<Badge>Label</Badge>

<!-- Outline -->
<Badge variant="outline">Info</Badge>
```

---

## 9. Period Selector (Tab Style)

```html
<div
  className="flex items-center p-0.5 xs:p-1 rounded-lg xs:rounded-xl"
  style={{ backgroundColor: 'rgba(141, 45, 83, 0.06)' }}
>
  {periods.map((period) => (
    <button
      key={period}
      className="flex-1 xs:flex-none px-2.5 xs:px-4 py-1 xs:py-1.5 rounded-md xs:rounded-lg text-xs xs:text-sm font-medium transition-all"
      style={{
        backgroundColor: isActive ? colors.wine : 'transparent',
        color: isActive ? '#ffffff' : colors.wineTextMuted,
      }}
    >
      {period}
    </button>
  ))}
</div>
```

---

## 10. Ícones SVG Customizados

### 10.1 Padrão de Ícone

```tsx
const CustomIcon: LucideIcon = ({ className, ...props }) => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 24 24"
    width="24"
    height="24"
    fill="none"
    stroke="currentColor"
    strokeWidth="1.5"
    strokeLinecap="round"
    strokeLinejoin="round"
    className={className}
    {...props}
  >
    {/* paths */}
  </svg>
);
```

---

## 11. Dark Mode

### 11.1 Variáveis CSS para Dark Mode

- Background muda para `336 78% 18%`
- Primary muda para orange `34 94% 51%`
- Textos ficam brancos

---

## 12. Animações

### 12.1 Animações Definidas

| Nome | Duração | Uso |
|------|---------|-----|
| accordion-down | 0.2s | Abertura de accordion |
| accordion-up | 0.2s | Fechamento de accordion |
| blink | 2s | Indicadores pulsantes |
| border-blink | 2s | Bordas pulsantes |

---

## 13. Checklist para Novas Páginas

- [ ] Usar container principal com border e rounded-xl
- [ ] Aplicar paleta wine para cards de estatísticas
- [ ] Usar breakpoint `xs:` para ajustes mobile
- [ ] Manter padrão de padding responsivo
- [ ] Usar inline styles para cores do tema wine
- [ ] Seguir hierarquia tipográfica
- [ ] Usar componentes shadcn/ui existentes
- [ ] Manter hover states consistentes
