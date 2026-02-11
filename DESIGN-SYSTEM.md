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

## 7. Inputs

### 7.1 Input com Ícone (Light Mode)

```tsx
<div>
  <label
    htmlFor="input-id"
    className="mb-1.5 block text-xs lg:text-sm font-medium pl-2.5 text-gray-700"
  >
    {label}
  </label>
  <div className="relative">
    <div className="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3 text-gray-400">
      <Icon className="h-4 w-4" />
    </div>
    <input
      id="input-id"
      type="text"
      placeholder="Placeholder..."
      className="w-full py-2 pl-9 pr-3 text-xs xs:py-2.5 xs:pl-10 xs:pr-4 xs:text-sm lg:py-3 lg:text-base transition focus:outline-none rounded-2xl border-2! border-gray-300! bg-gray-50 text-gray-900 placeholder-gray-400 hover:border-[#520B27]! focus:border-[#520B27]! focus:ring-1 focus:ring-[#3a0718]/30"
    />
  </div>
</div>
```

### 7.2 Input com Ícone (Dark Mode)

```tsx
<input
  className="w-full py-2 pl-9 pr-3 text-xs xs:py-2.5 xs:pl-10 xs:pr-4 xs:text-sm lg:py-3 lg:text-base transition focus:outline-none rounded-lg bg-gray-900/50 text-gray-100 placeholder-gray-500 ring-1 ring-gray-600/30 focus:ring-2 focus:ring-gray-400/50"
/>
```

### 7.3 Input com Toggle de Visibilidade (Password)

```tsx
<div className="relative">
  <div className="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3 text-gray-400">
    <LockIcon className="h-4 w-4" />
  </div>
  <input
    type={showPassword ? 'text' : 'password'}
    className="w-full py-2 pl-9 pr-10 text-xs xs:py-2.5 xs:pl-10 xs:pr-11 xs:text-sm lg:py-3 lg:text-base transition focus:outline-none rounded-2xl border-2! border-gray-300! bg-gray-50 text-gray-900 placeholder-gray-400 hover:border-[#520B27]! focus:border-[#520B27]! focus:ring-1 focus:ring-[#3a0718]/30"
  />
  <button
    type="button"
    onClick={togglePasswordVisibility}
    className="absolute inset-y-0 right-0 flex cursor-pointer items-center pr-3 transition text-gray-400 hover:text-[#520B27]"
  >
    {showPassword ? <EyeOffIcon className="h-4 w-4" /> : <EyeIcon className="h-4 w-4" />}
  </button>
</div>
```

### 7.4 Especificações do Input

| Propriedade | Light Mode | Dark Mode |
|-------------|------------|-----------|
| Border radius | `rounded-2xl` | `rounded-lg` |
| Border | `border-2! border-gray-300!` | `ring-1 ring-gray-600/30` |
| Background | `bg-gray-50` | `bg-gray-900/50` |
| Texto | `text-gray-900` | `text-gray-100` |
| Placeholder | `placeholder-gray-400` | `placeholder-gray-500` |
| Hover | `hover:border-[#520B27]!` | — |
| Focus | `focus:border-[#520B27]! focus:ring-1 focus:ring-[#3a0718]/30` | `focus:ring-2 focus:ring-gray-400/50` |
| Padding (base) | `py-2 pl-9 pr-3` | `py-2 pl-9 pr-3` |
| Padding (xs) | `xs:py-2.5 xs:pl-10 xs:pr-4` | `xs:py-2.5 xs:pl-10 xs:pr-4` |
| Padding (lg) | `lg:py-3` | `lg:py-3` |
| Font size | `text-xs xs:text-sm lg:text-base` | `text-xs xs:text-sm lg:text-base` |

---

## 8. Botões

### 8.1 Variantes (shadcn/ui)

| Variante | Uso | Classes |
|----------|-----|---------|
| default | Ação primária | `bg-primary text-primary-foreground` |
| outline | Ação secundária | `border border-input hover:bg-[#8D2D53] hover:text-white` |
| ghost | Ação terciária | `hover:bg-[#8D2D53] hover:text-white` |
| destructive | Ação perigosa | `bg-destructive text-destructive-foreground` |

### 8.2 Tamanhos (shadcn/ui)

| Tamanho | Classes |
|---------|---------|
| default | `h-10 px-4 py-2` |
| sm | `h-9 rounded-md px-3` |
| lg | `h-11 rounded-md px-8` |
| icon | `h-10 w-10` |

### 8.3 Botão Primário com Animação Glow (Auth)

Botão full-width com fundo escuro e efeito de glow animado no hover. Usado como CTA principal em páginas de autenticação.

```tsx
<button
  type="submit"
  disabled={loading}
  aria-busy={loading}
  className="auth-btn-glow relative mt-6 inline-flex w-full cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-full px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm lg:py-3 lg:text-base font-semibold text-white transition-all disabled:cursor-not-allowed disabled:opacity-50"
>
  <div className="auth-btn-glow__loader absolute inset-0 z-1 bg-transparent">
    <div className="auth-btn-glow__glow absolute inset-0" />
  </div>
  <span className="relative z-2 flex items-center gap-2 font-semibold select-none">
    <Icon className="h-4 w-4" />
    {label}
  </span>
</button>
```

#### CSS necessário (`index.css`)

```css
@keyframes auth-glow-move {
  0%   { transform: translate(-55%); }
  100% { transform: translate(55%); }
}

@keyframes auth-glow-pulse {
  0%, 100% { opacity: 0; }
  15%      { opacity: 1; }
  65%      { opacity: 0; }
}

.auth-btn-glow {
  background: linear-gradient(135deg, rgb(26, 26, 26) 0%, rgb(10, 10, 10) 100%);
  border: 1px solid rgba(255, 255, 255, 0.1);
  box-shadow:
    0 8px 32px rgba(0, 0, 0, 0.4),
    inset 0 1px 0 rgba(255, 255, 255, 0.1),
    inset 0 -1px 0 rgba(0, 0, 0, 0.5);
  transition: all 0.4s cubic-bezier(0.4, 0, 0.2, 1);
}

.auth-btn-glow:hover:not(:disabled) {
  transform: translateY(-2px);
  box-shadow:
    0 12px 40px rgba(0, 0, 0, 0.5),
    inset 0 1px 0 rgba(255, 255, 255, 0.15),
    inset 0 -1px 0 rgba(0, 0, 0, 0.6),
    0 0 0 2px rgba(244, 63, 94, 0.3);
}

.auth-btn-glow__loader {
  mask: repeating-linear-gradient(90deg, transparent 0, transparent 6px, black 7px, black 8px);
}

.auth-btn-glow__glow {
  background-image:
    radial-gradient(circle at 50% 50%, #f43f5e 0%, transparent 50%),
    radial-gradient(circle at 45% 45%, #ef4444 0%, transparent 45%),
    radial-gradient(circle at 55% 55%, #fb7185 0%, transparent 45%),
    radial-gradient(circle at 45% 55%, #f87171 0%, transparent 45%),
    radial-gradient(circle at 55% 45%, #dc2626 0%, transparent 45%);
  mask: radial-gradient(circle at 50% 50%, transparent 0%, transparent 10%, black 25%);
  opacity: 0;
}

.auth-btn-glow:hover .auth-btn-glow__glow {
  opacity: 1;
  animation:
    auth-glow-move 2s infinite alternate cubic-bezier(0.6, 0.8, 0.5, 1),
    auth-glow-pulse 4s infinite cubic-bezier(0.6, 0.8, 0.5, 1);
  filter: drop-shadow(0 0 8px rgba(244, 63, 94, 0.6));
}
```

### 8.4 Botão Secundário (Auth)

Botão outline full-width com elevação no hover. Usado como ação alternativa (ex: "Entrar com Magic Link", "Voltar").

```tsx
{/* Light Mode */}
<button
  type="button"
  className="inline-flex w-full cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-full px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm lg:py-3 lg:text-base font-semibold transition-all border! border-gray-300! bg-white text-[#520B27] shadow-sm hover:-translate-y-0.5 hover:border-gray-400! hover:shadow-md"
>
  <Icon className="h-4 w-4" />
  {label}
</button>

{/* Dark Mode — reutiliza a classe auth-btn-glow */}
<button
  type="button"
  className="auth-btn-glow inline-flex w-full cursor-pointer items-center justify-center gap-2 overflow-hidden rounded-full px-4 py-2 text-xs xs:px-5 xs:py-2.5 xs:text-sm lg:py-3 lg:text-base font-semibold text-white transition-all"
>
  <Icon className="h-4 w-4" />
  {label}
</button>
```

### 8.5 Especificações dos Botões Auth

| Propriedade | Primário (Glow) | Secundário (Light) | Secundário (Dark) |
|-------------|-----------------|---------------------|---------------------|
| Border radius | `rounded-full` | `rounded-full` | `rounded-full` |
| Background | Gradient escuro | `bg-white` | Gradient escuro (via `auth-btn-glow`) |
| Texto | `text-white` | `text-[#520B27]` | `text-white` |
| Border | `rgba(255,255,255,0.1)` | `border-gray-300!` | `rgba(255,255,255,0.1)` |
| Hover | `translateY(-2px)` + glow | `translateY(-0.5)` + shadow | `translateY(-2px)` + glow |
| Padding (base) | `px-4 py-2` | `px-4 py-2` | `px-4 py-2` |
| Padding (xs) | `xs:px-5 xs:py-2.5` | `xs:px-5 xs:py-2.5` | `xs:px-5 xs:py-2.5` |
| Padding (lg) | `lg:py-3` | `lg:py-3` | `lg:py-3` |
| Font | `font-semibold text-xs xs:text-sm lg:text-base` | Idem | Idem |
| Disabled | `disabled:cursor-not-allowed disabled:opacity-50` | — | — |

---

## 9. Badges

```html
<!-- Success -->
<Badge variant="success">Ativo</Badge>

<!-- Default (Wine) -->
<Badge>Label</Badge>

<!-- Outline -->
<Badge variant="outline">Info</Badge>
```

---

## 10. Period Selector (Tab Style)

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

## 11. Ícones SVG Customizados

### 11.1 Padrão de Ícone

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

## 12. Dark Mode

### 12.1 Variáveis CSS para Dark Mode

- Background muda para `336 78% 18%`
- Primary muda para orange `34 94% 51%`
- Textos ficam brancos

---

## 13. Animações

### 13.1 Animações Definidas

| Nome | Duração | Uso |
|------|---------|-----|
| accordion-down | 0.2s | Abertura de accordion |
| accordion-up | 0.2s | Fechamento de accordion |
| blink | 2s | Indicadores pulsantes |
| border-blink | 2s | Bordas pulsantes |

---

## 14. Checklist para Novas Páginas

- [ ] Usar container principal com border e rounded-xl
- [ ] Aplicar paleta wine para cards de estatísticas
- [ ] Usar breakpoint `xs:` para ajustes mobile
- [ ] Manter padrão de padding responsivo
- [ ] Usar inline styles para cores do tema wine
- [ ] Seguir hierarquia tipográfica
- [ ] Usar componentes shadcn/ui existentes
- [ ] Manter hover states consistentes
