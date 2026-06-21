---
# DESIGN.md — визуальная идентичность AshyqQala.kz
# Конвенция: Google Labs design.md spec (https://github.com/google-labs-code/design.md)
name: AshyqQala.kz
description: >-
  Гражданская платформа прозрачности госзакупок Казахстана (дороги и водоснабжение,
  пилот в Астане). Карта-центричный, деловитый и плотный интерфейс с заметным —
  но вкусовым — казахстанским характером. Палитра «Открытый город / Баланс»
  (голубой #1496C0 + золото #F0B429). Светлая и тёмная темы. Эмоция: доверие +
  гражданская сила. Флаг = нейтральный «сигнал, требующий проверки», НЕ обвинение.
status: final
created: 2026-06-18
updated: 2026-06-19
project: AshyqQala.kz

# ----------------------------------------------------------------------------
# COLORS — плоский объект, kebab-case. Светлая тема — базовые токены;
# тёмная тема — те же роли с суффиксом -dark (ручной переключатель).
# Источник палитры: Тема 4 «Открытый город / Баланс» (mockups/color-themes-1.html).
# Контрасты проверены против WCAG 2.1 AA (см. раздел Colors в теле).
# ОБЛАСТЬ ПРОВЕРКИ: проверены текстовые пары (текст-на-фоне) И графический контраст
# несущих UI-элементов (focus-ring ≥3:1, заливка выбранного чипа ≥3:1, ссылка на ОБОИХ
# фонах ≥4.5:1). Тонкие декоративные границы (border #D2E4EC = 1.31:1 на белом) НЕ несут
# смысл и допустимы; статус никогда не кодируется одним лишь цветом (см. map-marker-*).
# ----------------------------------------------------------------------------
colors:
  # --- Бренд / акцент ---
  primary: '#1496C0'              # голубой флага — бренд, маркеры карты, активные иконки
  primary-strong: '#0F7DA3'      # затемнённый голубой — заливка кнопок и ссылки (AA на тексте)
  primary-soft: '#DCEFF6'        # светлая голубая заливка (чипы, мини-иконки)
  on-primary: '#FFFFFF'          # текст/иконка на голубой заливке
  accent: '#F0B429'              # золото — орнамент, акцентные линии, выделенный маркер
  accent-soft: '#FBEFC9'         # мягкая золотая заливка
  on-accent: '#16242C'           # тёмный текст на золоте (AA, 8.5:1)

  # --- Семантика данных ---
  signal-attention: '#E09915'    # ФЛАГ — нейтральный амбер «требует проверки» (НЕ алый)
  signal-attention-bg: '#FFF6E0' # фон бейджа флага (светлая тема)
  signal-attention-fg: '#7A560D' # текст на фоне бейджа флага (AA, 6.2:1)
  signal-attention-border: '#F2D58C'
  confirmed: '#2C8C7A'           # «подтверждено / из первоисточника» — спокойный сине-зелёный (метка/иконка)
  confirmed-text: '#1F7A6B'      # тот же смысл как ТЕКСТ на белом (AA, 5.2:1)
  confirmed-soft: '#D7EEE8'

  # --- Состояния данных / нейтраль ---
  muted: '#8893A0'               # decorative no-data: точки, иконки, штрих-границы
  muted-text: '#5A6672'          # «нет данных» / «недостаточно…» как ЧИТАЕМЫЙ текст (AA, 5.9:1)
  muted-soft: '#EBEFF2'          # фон-плашка состояния «недостаточно данных»

  # --- Поверхности и текст ---
  surface: '#FFFFFF'             # карточки, листы, попап-превью
  surface-sunken: '#EEF6F9'      # фон приложения / поля ввода (голубоватый)
  border: '#D2E4EC'             # 1px разделители и контуры (декоративные, не несут смысл)
  text-primary: '#16242C'        # основной текст (AA на surface, 15.9:1)
  text-secondary: '#4E5E69'      # вторичный текст / подписи (AA на surface, 6.7:1)
  link: '#0F7DA3'                # ссылка на белом surface (AA, 4.68:1) — НЕ для фона sunken
  link-on-sunken: '#0C6585'      # ссылка-первоисточник, AA на ОБОИХ фонах: white 6.53:1, sunken 5.97:1
  focus-ring: '#0C6585'          # видимый фокус-ринг: white 6.53:1, sunken 5.97:1 (≥3:1, граф. контраст 1.4.11)

  # ===== ТЁМНАЯ ТЕМА (-dark) — ручной переключатель =====
  primary-dark: '#3BB3D6'        # осветлённый голубой — бренд/маркеры/иконки на тёмном
  primary-strong-dark: '#5FC6E3' # заливка кнопок на тёмном
  primary-soft-dark: '#173A47'   # светлая голубая заливка-эквивалент на тёмном
  on-primary-dark: '#06222B'     # тёмный текст на осветлённой голубой кнопке (AA, 6.8:1)
  accent-dark: '#F5C14E'         # золото, читаемое на тёмном
  accent-soft-dark: '#3A2E12'
  on-accent-dark: '#16242C'

  signal-attention-dark: '#E8A93A'      # флаг-амбер на тёмном (точка/иконка, 7.7:1)
  signal-attention-bg-dark: '#33280F'   # фон бейджа флага (тёмная тема)
  signal-attention-fg-dark: '#F0CE82'   # текст бейджа флага (AA, 9.6:1)
  signal-attention-border-dark: '#5C4A1E'
  confirmed-dark: '#3FB39C'             # verified на тёмном (6.2:1)
  confirmed-text-dark: '#3FB39C'
  confirmed-soft-dark: '#16352F'

  muted-dark: '#6B7785'                 # decorative no-data на тёмном
  muted-text-dark: '#9AA6B2'            # читаемый no-data текст (AA, 6.4:1)
  muted-soft-dark: '#1E2C34'

  surface-dark: '#16242C'               # карточки/листы/попап на тёмном
  surface-sunken-dark: '#0E1A21'        # фон приложения тёмной темы
  border-dark: '#2A3D47'
  text-primary-dark: '#EAF1F4'          # основной текст (AA, 13.9:1)
  text-secondary-dark: '#AAB8C0'        # вторичный текст (AA, 7.8:1)
  link-dark: '#56C0E0'                  # ссылка на тёмном surface (AA, 7.57:1)
  link-on-sunken-dark: '#5BC4E4'        # ссылка, AA на ОБОИХ тёмных фонах: surface 7.91:1, sunken 8.80:1
  focus-ring-dark: '#5FC6E3'            # фокус-ринг тёмной темы: surface 8.07:1, sunken 8.98:1 (≥3:1)

# ----------------------------------------------------------------------------
# TYPOGRAPHY — web-safe стек с ПОЛНЫМ покрытием расширенной казахской кириллицы
# (ә ғ қ ң ө ұ ү і h) и русской кириллицы. Шрифты-кандидаты ниже все покрывают
# KZ-глифы; стек начинается с системных, чтобы гарантировать рендеринг и скорость.
# fontFamily повторяется в каждой роли намеренно (плоская резолюция токенов).
# ----------------------------------------------------------------------------
typography:
  # Основной системный стек (покрывает KZ кириллицу: Segoe UI / Roboto / SF имеют полный набор)
  display:
    fontFamily: '"Inter", "Segoe UI", Roboto, "Helvetica Neue", Arial, system-ui, sans-serif'
    fontSize: 24px
    fontWeight: '700'
    lineHeight: '1.2'
    letterSpacing: -0.01em
  heading:
    fontFamily: '"Inter", "Segoe UI", Roboto, "Helvetica Neue", Arial, system-ui, sans-serif'
    fontSize: 17px
    fontWeight: '700'
    lineHeight: '1.25'
    letterSpacing: -0.005em
  subheading:
    fontFamily: '"Inter", "Segoe UI", Roboto, "Helvetica Neue", Arial, system-ui, sans-serif'
    fontSize: 15px
    fontWeight: '600'
    lineHeight: '1.3'
  amount:
    fontFamily: '"Inter", "Segoe UI", Roboto, "Helvetica Neue", Arial, system-ui, sans-serif'
    fontSize: 21px
    fontWeight: '800'
    lineHeight: '1.15'
    letterSpacing: -0.01em
  body:
    fontFamily: '"Inter", "Segoe UI", Roboto, "Helvetica Neue", Arial, system-ui, sans-serif'
    fontSize: 14px
    fontWeight: '400'
    lineHeight: '1.45'
  body-strong:
    fontFamily: '"Inter", "Segoe UI", Roboto, "Helvetica Neue", Arial, system-ui, sans-serif'
    fontSize: 14px
    fontWeight: '600'
    lineHeight: '1.45'
  meta:
    fontFamily: '"Inter", "Segoe UI", Roboto, "Helvetica Neue", Arial, system-ui, sans-serif'
    fontSize: 12px
    fontWeight: '500'
    lineHeight: '1.35'
  label-caps:
    fontFamily: '"Inter", "Segoe UI", Roboto, "Helvetica Neue", Arial, system-ui, sans-serif'
    fontSize: 9.5px
    fontWeight: '700'
    lineHeight: '1.2'
    letterSpacing: 0.1em
  mono:
    fontFamily: 'ui-monospace, "SF Mono", "Roboto Mono", Menlo, Consolas, monospace'
    fontSize: 12px
    fontWeight: '500'
    lineHeight: '1.4'

# ----------------------------------------------------------------------------
# ROUNDED — радиусы. Личность = Направление B (умеренный радиус карточек 12px).
# ----------------------------------------------------------------------------
rounded:
  none: 0px
  sm: 6px         # поля ввода, чипы, мелкие плашки
  md: 10px        # кнопки, попап-превью, бейдж флага
  DEFAULT: 12px   # карточка контракта / подрядчика (центр личности B)
  card: 12px      # алиас DEFAULT для читаемости в ссылках компонентов
  lg: 18px        # нижний лист (верхние углы), модальные слои
  xl: 26px        # крупные декоративные поверхности (редко)
  full: 9999px    # пилюли: бейдж флага-таблетка, метка маркера, переключатели

# ----------------------------------------------------------------------------
# SPACING — база 4px. Средняя плотность (Направление B): плотно, но дышит.
# ----------------------------------------------------------------------------
spacing:
  '0': 0px
  '1': 4px
  '2': 8px
  '3': 12px
  '4': 16px
  '5': 20px
  '6': 24px
  '8': 32px
  '10': 40px
  gutter: 16px            # внутренний отступ карточек/листов
  margin-mobile: 16px     # боковые поля экрана (mobile-first)
  section-gap: 24px       # промежуток между крупными блоками карточки
  row-gap: 6px            # между строками метаданных (dt/dd)
  tap-target: 44px        # минимальный размер интерактивной цели (доступность)
  focus-ring-width: 2px   # толщина фокус-ринга
  focus-ring-offset: 2px  # отступ фокус-ринга от края элемента

# ----------------------------------------------------------------------------
# COMPONENTS — визуальные спецификации. Поведение → EXPERIENCE.md.
# ----------------------------------------------------------------------------
components:
  contract-card:
    background: '{colors.surface}'
    border: '1px solid {colors.border}'
    radius: '{rounded.card}'
    padding: '{spacing.gutter}'
    title: '{typography.heading}'
    amount: '{typography.amount}'
    amount-color: '{colors.primary-strong}'
    meta-label: '{colors.text-secondary}'
    meta-value: '{colors.text-primary}'
    shadow: '0 2px 8px rgba(22,36,44,.06)'
  flag-badge:
    background: '{colors.signal-attention-bg}'
    foreground: '{colors.signal-attention-fg}'
    border: '1px solid {colors.signal-attention-border}'
    dot: '{colors.signal-attention}'
    glyph: '! ({colors.signal-attention-fg})'          # не-цветовой признак флага (паритет с {components.map-marker-flagged})
    radius: '{rounded.full}'
    typography: '{typography.meta}'
    padding: '7px 10px'
    min-height: '{spacing.tap-target}'                 # интерактивна (role=button) → хитбокс ≥44px
    focus-ring: '{components.focus-ring}'
  map-marker-point:                         # обычный объект (без флага, не выбран)
    fill: '{colors.primary}'
    stroke: '{colors.surface}'
    stroke-width: '2px'
    shape: 'teardrop (radius 50% 50% 50% 2px)'
    glyph: 'none'                            # пустой пин = первичный не-цветовой признак «обычный»
    min-hit-area: '{spacing.tap-target}'    # хитбокс ≥44px вокруг визуально мелкого пина
    focus-ring: '{components.focus-ring}'
  map-marker-flagged:                        # есть флаг «сигнал, требующий проверки»
    fill: '{colors.signal-attention}'
    stroke: '{colors.surface}'
    stroke-width: '2px'
    shape: 'teardrop (radius 50% 50% 50% 2px)'
    glyph: '! (восклицательный, {colors.on-accent})'  # ПЕРВИЧНЫЙ не-цветовой признак (амбер — вторичен)
    glyph-source: 'совпадает с «!» в {components.flag-badge}'
    min-hit-area: '{spacing.tap-target}'
    z-priority: '2'                          # рисуется выше обычного при перекрытии (см. EXPERIENCE.md)
  map-marker-confirmed:                      # данные из первоисточника (см. {components.confirmed-tag})
    fill: '{colors.confirmed}'
    stroke: '{colors.surface}'
    stroke-width: '2px'
    shape: 'teardrop (radius 50% 50% 50% 2px)'
    glyph: '✓ (галочка, {colors.surface})'   # не-цветовой признак «подтверждено»
    min-hit-area: '{spacing.tap-target}'
  map-marker-selected:                       # ВЫБРАННЫЙ (любой статус) — отличается ФОРМОЙ, не только цветом
    fill: '{colors.accent}'
    stroke: '{colors.surface}'
    stroke-width: '3px'                      # утолщённая обводка = не-цветовой признак выбора
    shape: 'teardrop увеличенный ×1.25 + двойное кольцо-обводка'
    ring: '2px {colors.primary-strong}'      # отдельное кольцо помимо белой обводки
    glyph: 'сохраняется глиф исходного статуса (!/✓/пусто)'
    min-hit-area: '{spacing.tap-target}'
    z-priority: '3'                          # выбранный всегда поверх флага и обычного
  map-line:
    stroke: '{colors.primary}'
    stroke-width: '6px'
    casing: '{colors.surface}'
    stroke-dasharray: '5 6'
    stroke-selected: '{colors.accent}'       # ВЫБРАННАЯ линия — золотой stroke (паритет с {components.map-marker-selected})
    casing-selected: '{colors.primary-strong}'
  map-cluster:
    background: '{colors.primary}'
    foreground: '{colors.on-primary}'
    halo: 'rgba(20,150,192,.25)'
    radius: '{rounded.full}'
    typography: '{typography.body-strong}'
    count-badge: 'число объектов белым ({colors.on-primary}) по центру круга'  # счётчик кластера
    flag-indicator: 'если в кластере есть флаг-объекты — амбер-кольцо {colors.signal-attention} + «!»-точка'
    min-hit-area: '{spacing.tap-target}'
  preview-sheet:
    background: '{colors.surface}'
    radius: '{rounded.lg} {rounded.lg} 0 0'
    handle: '{colors.border}'
    shadow: '0 -8px 22px rgba(22,36,44,.12)'
    padding: '{spacing.gutter}'
  filter-chip:
    background: '{colors.surface-sunken}'
    foreground: '{colors.text-secondary}'
    border: '1px solid {colors.border}'
    radius: '{rounded.full}'
    typography: '{typography.meta}'
    min-height: '{spacing.tap-target}'                 # хитбокс ≥44px независимо от 12px-кегля
    # ВЫБРАННОЕ состояние: насыщенная заливка + белый текст + не-цветовая галочка
    selected-background: '{colors.link-on-sunken}'     # #0C6585: белый текст 6.53:1, фон-vs-невыбранный 5.97:1
    selected-foreground: '{colors.on-primary}'         # белый (AA, 6.53:1 на заливке)
    selected-mark: '✓ (галочка перед текстом) + bold'  # НЕ-цветовой признак выбора (WCAG 1.4.1)
    selected-background-dark: '{colors.primary-dark}'  # #3BB3D6: тёмный текст 6.77:1, фон-vs-невыбранный 7.24:1
    selected-foreground-dark: '{colors.on-primary-dark}'  # #06222B
    focus-ring: '{components.focus-ring}'
  data-state-nodata:
    foreground: '{colors.muted-text}'
    typography: '{typography.body}'
    style: 'italic'
    glyph: '— (тире/прочерк-глиф)'                     # не-цветовой различитель состояния «нет данных»
  data-state-insufficient:
    background: '{colors.muted-soft}'
    foreground: '{colors.muted-text}'
    border: '1px solid {colors.border}'                # СПЛОШНОЙ нейтральный бордюр (не dashed — не «черновик/ошибка»)
    radius: '{rounded.sm}'
    typography: '{typography.meta}'
    glyph: '≈ (приблизительно) или «<5»'               # не-цветовой различитель «недостаточно данных»
    tone: 'информативно-нейтральный, НЕ warning/error (не красный, не «битый» вид)'
  data-state-ungeocoded:
    foreground: '{colors.muted-text}'
    icon: '{colors.muted}'
    icon-glyph: 'пин-перечёркнутый / «без точки»'      # не-цветовой различитель «не геопривязан»
    typography: '{typography.meta}'
    tone: 'информативно-нейтральный, не дефект объекта'
  source-link:
    foreground: '{colors.link-on-sunken}'              # AA на ОБОИХ фонах (surface И surface-sunken)
    foreground-dark: '{colors.link-on-sunken-dark}'
    typography: '{typography.body-strong}'
    decoration: 'underline (offset 2px)'               # подчёркивание всегда — различимость помимо цвета
    focus-ring: '{components.focus-ring}'
  confirmed-tag:                                        # «данные из первоисточника» рядом с {components.source-link}
    foreground: '{colors.confirmed-text}'              # #1F7A6B (AA 5.2:1 на surface)
    foreground-dark: '{colors.confirmed-text-dark}'
    background: '{colors.confirmed-soft}'
    glyph: '✓ ({colors.confirmed})'                     # не-цветовой признак «подтверждено»
    radius: '{rounded.full}'
    typography: '{typography.meta}'
  focus-ring:                                           # ОБЯЗАТЕЛЕН на всех интерактивных целях (вкл. маркеры карты)
    color: '{colors.focus-ring}'                       # #0C6585 (≥3:1 на surface 6.53 и sunken 5.97)
    color-dark: '{colors.focus-ring-dark}'             # #5FC6E3 (≥3:1 на surface 8.07 и sunken 8.98)
    width: '{spacing.focus-ring-width}'                # 2px
    offset: '{spacing.focus-ring-offset}'              # 2px
    style: 'сплошное кольцо-обводка с офсетом; видимо в обеих темах; не полагается на нативный браузерный ринг'
  lang-switch:
    background: '{colors.surface-sunken}'
    border: '1px solid {colors.border}'
    radius: '{rounded.full}'
    active-background: '{colors.primary}'
    active-foreground: '{colors.on-primary}'
    typography: '{typography.label-caps}'
    min-height: '{spacing.tap-target}'                 # ≥44px интерактивной высоты, несмотря на 9.5px label-caps
    min-segment-width: '{spacing.tap-target}'          # каждый сегмент (KZ/RU) ≥44px по тапу
    focus-ring: '{components.focus-ring}'
  theme-switch:
    track: '{colors.border}'
    thumb: '{colors.surface}'
    icon-color: '{colors.text-secondary}'
    radius: '{rounded.full}'
    min-height: '{spacing.tap-target}'                 # ≥44px хитбокс тумблера
    min-width: '{spacing.tap-target}'
    focus-ring: '{components.focus-ring}'
  ornament-divider:
    color: '{colors.accent}'
    line: 'linear-gradient(90deg, transparent, {colors.border})'
    motif: 'қошқар мүйіз (парные завитки), stroke-width 1.6px, width ≈64px'
---

# DESIGN.md — AshyqQala.kz

Визуальная идентичность платформы. Документ — источник истины для визуала: при
конфликте с любым моком, импортом или скриншотом **побеждает DESIGN.md**.
Семантические имена токенов выше предназначены для ссылок из EXPERIENCE.md
синтаксисом `{path.to.token}` (например `{colors.signal-attention}`,
`{rounded.card}`, `{typography.amount}`).

## Brand & Style

AshyqQala.kz — «Открытый город». Это серьёзная карта-утилита для гражданина,
а не сухой госреестр и не фолк-фестиваль. Эстетическая поза — **деловитая
плотность с человеческим теплом**: ориентир по виду и ощущению — 2GIS (карта в
центре, насыщенно, но читаемо, узнаваемый акцентный цвет, ничего лишнего), а
характер — заметно казахстанский (палитра флага: небесно-голубой и золото; тонкий
орнамент қошқар мүйіз; простор степи в светлых фонах).

Выбранная личность — **Направление B «Открытый город / Баланс»**: средняя
плотность, умеренно скруглённые карточки ({rounded.card} = 12px), орнамент
қошқар мүйіз появляется **тонким мотивом-разделителем** (не фон-перегруз), золото
работает в акцентах кнопок и линий. Стрелка компаса — ровно между «официально» и «живо».
Сравнение трёх направлений: [mockups/directions-1.html](mockups/directions-1.html)
— выбрано Направление B.

Целевое чувство пользователя за первые секунды — **доверие + гражданская сила**,
не тревога и не обличение. Это диктует главный визуальный закон платформы:
**каждый риск-флаг — нейтральный «сигнал, требующий проверки», и его цвет
никогда не кричит «нарушение»**. Честность важнее вывода: нет данных —
показываем «нет данных», а не интерполируем.

## Colors

Палитра — Тема 4 «Открытый город / Баланс»: прохладный голубой как бренд, золото
как акцент, амбер как нейтральный сигнал. Полный набор задан для светлой и тёмной
тем (ручной переключатель `{components.theme-switch}`). Все текстовые пары
проверены против **WCAG 2.1 AA** (нормальный текст ≥4.5:1, крупный/жирный и
UI-компоненты ≥3:1). Цветовые пробы:
[mockups/color-themes-1.html](mockups/color-themes-1.html) — выбрана Тема 4.

**Бренд и акцент.**
- **Голубой `{colors.primary}` (#1496C0)** — лицо бренда: логотип-марка, маркеры
  карты, активные иконки, полилинии дорог. Как *заливка под белым текстом* он даёт
  3.41:1 — этого хватает для крупного/UI, но НЕ для мелкого текста, поэтому
  кнопочная заливка использует затемнённый **`{colors.primary-strong}` (#0F7DA3,
  4.7:1)**. Голубой — не цвет тревоги и не цвет «всё хорошо».
- **Золото `{colors.accent}` (#F0B429)** — казахстанский акцент: орнамент-разделитель,
  выделенный (выбранный) маркер карты, акцентные линии и подсветка первичного
  действия. На золоте всегда тёмный текст `{colors.on-accent}` (8.5:1). Золото —
  украшение и фокус, не семантика состояния.

**Семантика данных.**
- **Амбер `{colors.signal-attention}` (#E09915)** — единственный цвет риск-флага.
  Это *нейтральное внимание* «требует проверки», а не «нарушение». Бейдж флага —
  мягкий фон `{colors.signal-attention-bg}` + тёмно-янтарный текст
  `{colors.signal-attention-fg}` (6.2:1) + амбер-точка с глифом «!». Тревожно-алый под
  флаги **запрещён** (см. Do's and Don'ts). Амбер — **вторичный** канал: статус «есть
  флаг» несёт прежде всего **форма/глиф** маркера (см. ниже), а не цвет (WCAG 1.4.1).
- **Сине-зелёный `{colors.confirmed}` (#2C8C7A)** — «подтверждено / данные из
  первоисточника». Потребители: метка `{components.confirmed-tag}` рядом с
  `{components.source-link}` и маркер `{components.map-marker-confirmed}` (глиф «✓»).
  Как фоновая метка/иконка — #2C8C7A; как читаемый текст на белом —
  `{colors.confirmed-text}` (#1F7A6B, 5.2:1). Это НЕ «зелёный светофора»: зелёный
  светофор зарезервирован под будущий «светофор регионов» и к флагам/состояниям
  карточки не применяется. Признак «подтверждено» несёт глиф «✓», не один цвет.

**Нейтраль и состояния отсутствия данных.**
- **`{colors.muted}` (#8893A0)** — декоративная нейтраль (штрих-границы, точки,
  иконки). Намеренно низкоконтрастна и НЕ используется как читаемый текст.
- **`{colors.muted-text}` (#5A6672, 5.9:1)** — честные состояния как **текст**:
  «нет данных», «недостаточно сопоставимых данных», «объект не геопривязан».

Визуальный тон состояний нехватки данных — **приглушённо-информативный, НЕ ошибка**:
никакого красного, никакого «битого»/«черновикового» вида. Плашка «недостаточно
сопоставимых данных» `{components.data-state-insufficient}` использует **сплошной**
нейтральный бордюр `{colors.border}` (не штриховой — штрих читается как «черновик/
проблема» и окрашивает законную нехватку выборки как дефект). У каждого состояния — свой
**не-цветовой глиф** («—» для «нет данных», «≈»/«<5» для «недостаточно», перечёркнутый
пин для «не геопривязан»), чтобы состояния различались не только тоном (WCAG 1.4.1).

**Поверхности.** `{colors.surface}` (белый) — карточки и листы;
`{colors.surface-sunken}` (#EEF6F9, голубоватый «воздух степи») — фон приложения и
поля. `{colors.text-primary}` (15.9:1) и `{colors.text-secondary}` (6.7:1) — текст.

**Ссылки на двух фонах.** `{colors.link}` (#0F7DA3) корректен только на белом
`{colors.surface}` (4.68:1); на фоне приложения `{colors.surface-sunken}` он даёт
**4.28:1 — ниже AA**. Поскольку ссылка-первоисточник `{components.source-link}`
присутствует на каждой поверхности с доменными данными (в т.ч. на sunken), её цвет —
**`{colors.link-on-sunken}` (#0C6585)**, проверенный на ОБОИХ фонах: белый **6.53:1**,
sunken **5.97:1**. В тёмной теме аналогично — `{colors.link-on-sunken-dark}` (#5BC4E4):
surface **7.91:1**, sunken **8.80:1**. Ссылка всегда подчёркнута (различимость помимо цвета).

**Фокус.** Токен `{colors.focus-ring}` (#0C6585) — видимый фокус-ринг 2px с офсетом 2px
(`{components.focus-ring}`), обязателен на всех интерактивных целях, включая маркеры карты.
Графический контраст ≥3:1 на обеих темах: светлая — surface **6.53:1** / sunken **5.97:1**;
тёмная `{colors.focus-ring-dark}` (#5FC6E3) — surface **8.07:1** / sunken **8.98:1**.
Нативный браузерный ринг не используется (он теряется на голубоватом sunken).

**Тёмная тема.** Те же роли с суффиксом `-dark`. Голубой и золото осветлены
(`{colors.primary-dark}` #3BB3D6, `{colors.accent-dark}` #F5C14E) ради контраста на
тёмных поверхностях `{colors.surface-dark}` (#16242C) / `{colors.surface-sunken-dark}`
(#0E1A21). Гардрейл флага сохраняется: амбер `{colors.signal-attention-dark}`,
никакого алого. Все текстовые пары проверены AA (текст 7.6–15.5:1; кнопки 6.8:1;
флаг-бейдж 9.6:1; ссылка `{colors.link-on-sunken-dark}` 7.9–8.8:1). Графические
несущие сочетания: фокус-ринг `{colors.focus-ring-dark}` ≥8:1, заливка выбранного
чипа `{colors.primary-dark}` с текстом `{colors.on-primary-dark}` 6.77:1.

## Typography

Основной язык интерфейса — **казахский**, переключатель на русский. Поэтому
типографика **обязана покрывать расширенную казахскую кириллицу: ә ғ қ ң ө ұ ү і**
(и h), а также полный русский набор. Это жёсткое требование к выбору шрифта.

Семейство — `Inter` с системным фолбэком (`"Segoe UI", Roboto, "Helvetica Neue",
Arial, system-ui`). Inter и все шрифты фолбэк-стека имеют **полное покрытие
казахских и русских глифов**; стек начинается с системных гарантий, чтобы текст
рендерился даже без загрузки веб-шрифта (Айгуль на мобильном интернете на улице).
Один гротеск на весь интерфейс — деловито и утилитарно, в духе карты-инструмента;
декоративных/serif-шрифтов нет.

Рамп ролей (mobile-first):
- `{typography.display}` (24px/700) — заголовок страницы/района.
- `{typography.heading}` (17px/700) — заголовок карточки контракта (название объекта).
- `{typography.subheading}` (15px/600) — подзаголовки секций внутри карточки.
- `{typography.amount}` (21px/800) — сумма договора (₸); самый «весомый» элемент карточки.
- `{typography.body}` / `{typography.body-strong}` (14px) — основной текст и значения.
- `{typography.meta}` (12px/500) — подписи, метаданные, бейджи, состояния данных.
- `{typography.label-caps}` (9.5px/700, +0.1em, UPPERCASE) — над-метки
  («КОНТРАКТ · ДОРОГА»), переключатель языка KZ/RU.
- `{typography.mono}` — числовые/технические значения (BIN, ₸/км), где важна
  моноширинность.

Числа форматируются по локали: разрядность «240 млн ₸», KZ/RU подписи единиц.
Заголовки и суммы — с лёгким отрицательным трекингом для плотного «инструментального»
блока; label-caps — наоборот, разрежены.

## Layout & Spacing

Шкала на базе 4px: `{spacing.1}`…`{spacing.10}` (4/8/12/16/20/24/32/40px).
Плотность — **средняя** (Направление B): плотно, но дышит. Самые крупные зазоры —
`{spacing.section-gap}` (24px) между смысловыми блоками карточки; самые мелкие —
`{spacing.row-gap}` (6px) между строками метаданных.

Form-factor — **mobile-first веб** (React + Vite + MapLibre GL); нативного
приложения нет. Боковые поля экрана — `{spacing.margin-mobile}` (16px); внутренний
отступ карточек и листов — `{spacing.gutter}` (16px). Раскладка — одна колонка;
карта занимает основную высоту, контракт раскрывается нижним листом
`{components.preview-sheet}`. На широких экранах список/фильтры могут уходить в
боковую панель, но контентная колонка остаётся одноколоночной и читаемой.

Все интерактивные цели — не меньше `{spacing.tap-target}` (44px) по **интерактивной**
зоне, даже если визуальный размер меньше: переключатели `{components.lang-switch}` /
`{components.theme-switch}` (9.5px label-caps, но хитбокс ≥44px), фильтр-чипы, зум-контролы
и маркеры карты (расширенный хитбокс вокруг мелкого пина). Визуальный размер ≠ цель.

Все интерактивные цели несут видимый фокус-ринг `{components.focus-ring}` (см. Colors).
Сетка карты — живая и плавная как 2GIS (must); микроанимации UI — сдержанные и
функциональные, **уважают `prefers-reduced-motion`** (декоративное движение отключается;
поведение — в EXPERIENCE.md).

## Elevation & Depth

Глубина — сдержанная: иерархию задают раскладка и тон поверхностей, тени —
вспомогательны и мягки (никаких резких «материал»-теней).
- **Карточка контракта** `{components.contract-card}` — `0 2px 8px rgba(22,36,44,.06)`:
  едва приподнята над фоном `{colors.surface-sunken}`.
- **Нижний лист** `{components.preview-sheet}` — тень вверх `0 -8px 22px
  rgba(22,36,44,.12)`: лист «приходит снизу» поверх карты.
- **Попап-превью на карте** и **зум-контролы** — `0 10px 24px rgba(22,36,44,.22)` /
  `0 3px 8px rgba(22,36,44,.18)`: оторваны от карты, но без драмы.
- **Маркеры** — мелкая drop-shadow `0 3px 6px rgba(12,26,34,.30)` + белая обводка,
  чтобы читаться на любом тайле.

В тёмной теме тени почти невидимы — слои разделяет тон (`{colors.surface-dark}`
над `{colors.surface-sunken-dark}`) и `{colors.border-dark}`, а не усиленная тень.

## Shapes

Логика форм — Направление B: умеренное скругление, «деловито, но не казённо».
- `{rounded.sm}` (6px) — поля ввода, фильтр-чипы-прямоугольники, плашка
  «недостаточно данных».
- `{rounded.card}` / `{rounded.DEFAULT}` (12px) — карточка контракта и подрядчика
  (центр личности).
- `{rounded.md}` (10px) — кнопки, попап-превью, зум-контролы.
- `{rounded.lg}` (18px) — верхние углы нижнего листа и модальных слоёв.
- `{rounded.full}` — пилюли: бейдж флага, метка маркера, переключатели языка и темы,
  выбранные фильтр-чипы.

Маркер карты — каплевидный (`50% 50% 50% 2px` с поворотом), узнаваемый силуэт пина.
**Статус маркера различается формой/глифом, а не только цветом** (WCAG 1.4.1) — цвет
вторичен:
- **обычный** `{components.map-marker-point}` — пустой голубой пин (без глифа);
- **есть флаг** `{components.map-marker-flagged}` — пин с глифом «!» (тот же «!», что в
  бейдже флага); амбер — вторичный признак;
- **подтверждено** `{components.map-marker-confirmed}` — пин с глифом «✓» (сине-зелёный);
- **выбран** `{components.map-marker-selected}` — увеличенный пин ×1.25 с **двойным
  кольцом-обводкой**, сохраняет глиф своего статуса; золото — вторичный признак;
- **без точки** — объект не на карте вовсе (см. `{components.data-state-ungeocoded}`),
  его статус «без точки» — это отсутствие маркера, а не другой цвет.

Изображения и тайлы следуют радиусу контейнера. Орнамент қошқар мүйіз — линейный
мотив-разделитель `{components.ornament-divider}`, не заливка и не фон-паттерн на
рабочих экранах.

## Components

Описание визуальное; поведение — в EXPERIENCE.md. Ключевые экраны-моки иллюстрируют
компоненты в сборе (светлая и тёмная темы):
[mockups/key-map.html](mockups/key-map.html) — маркеры, кластер, превью-лист;
[mockups/key-contract-card.html](mockups/key-contract-card.html) — карточка контракта,
бейдж флага, состояния данных;
[mockups/key-flag-methodology.html](mockups/key-flag-methodology.html) — модалка методики флага;
[mockups/key-district.html](mockups/key-district.html) — фильтр-чипы и их состояния;
[mockups/key-contractor-card.html](mockups/key-contractor-card.html) — карточка подрядчика;
[mockups/key-search-filters.html](mockups/key-search-filters.html) — поиск и фильтры;
[mockups/key-telegram-bot.html](mockups/key-telegram-bot.html) — Telegram-бот.

- **Карточка контракта** `{components.contract-card}` — белая поверхность, бордюр
  `{colors.border}`, радиус 12px. Над-метка label-caps («КОНТРАКТ · ДОРОГА»),
  заголовок-название `{typography.heading}`, сумма `{typography.amount}` цветом
  `{colors.primary-strong}`, далее метаданные (заказчик / подрядчик / способ /
  срок) парами dt→dd. Пример строки состояния: «Срок завершения — *нет данных*».
- **Бейдж флага (нейтральный)** `{components.flag-badge}` — пилюля на мягком амбер-фоне
  с янтарным текстом и амбер-точкой/иконкой «!». Текст всегда «… — сигнал, требующий
  проверки» (напр. «Единственный участник — сигнал, требующий проверки»). Никогда не
  алый, никогда «нарушение/виновен».
- **Маркеры карты — статус формой/глифом, цвет вторичен (WCAG 1.4.1).** Обычный
  `{components.map-marker-point}` — пустой голубой пин. С флагом
  `{components.map-marker-flagged}` — амбер-пин с глифом «!» (паритет с бейджем). С
  подтверждением `{components.map-marker-confirmed}` — пин с «✓». Выбранный
  `{components.map-marker-selected}` — увеличенный пин с двойным кольцом-обводкой
  (золото — вторично), сохраняет глиф статуса. У всех — белая обводка для читаемости на
  любом тайле и хитбокс ≥`{spacing.tap-target}`. Полилиния `{components.map-line}` —
  голубая трасса с белым casing и пунктиром; **выбранная** линия — золотой stroke
  (`stroke-selected`). Кластер `{components.map-cluster}` — голубой круг с **бейджем-
  счётчиком** объектов (белое число по центру) и мягким гало; если в кластере есть
  флаг-объекты — амбер-кольцо с «!»-индикатором.
- **Нижний лист превью** `{components.preview-sheet}` — белый лист со скруглёнными
  верхними углами (18px), «ручкой»-хваткой `{colors.border}` по центру; внутри —
  компактная карточка контракта и ссылка-первоисточник.
- **Чипы фильтров** `{components.filter-chip}` — пилюли (хитбокс ≥`{spacing.tap-target}`):
  невыбранные на `{colors.surface-sunken}` с текстом `{colors.text-secondary}`; выбранные
  на **насыщенной** заливке `{colors.link-on-sunken}` (#0C6585) с **белым** текстом
  `{colors.on-primary}` (6.53:1) и **галочкой «✓»** перед текстом (не-цветовой признак
  выбора). Заливка выбранного отличается от невыбранного фона 5.97:1 (раньше было 1.08:1).
  В тёмной теме — заливка `{colors.primary-dark}` + текст `{colors.on-primary-dark}` (6.77:1).
- **Метки состояний данных — приглушённо-нейтральные, НЕ ошибка**, у каждого свой глиф:
  «нет данных» `{components.data-state-nodata}` (курсив, `{colors.muted-text}`, глиф «—»);
  «недостаточно сопоставимых данных» `{components.data-state-insufficient}`
  (плашка `{colors.muted-soft}` со **сплошным** нейтральным бордюром, глиф «≈»/«<5» —
  без штриха и без красного, не «битый» вид); «объект не геопривязан»
  `{components.data-state-ungeocoded}` (мета-текст + иконка-перечёркнутый-пин `{colors.muted}`).
- **Кнопка-первоисточник** `{components.source-link}` — ссылка цветом
  `{colors.link-on-sunken}` (AA на surface И surface-sunken), всегда подчёркнута, со
  стрелкой ↗: «Первоисточник: goszakup.gov.kz ↗». Рядом может стоять метка
  `{components.confirmed-tag}` «данные из первоисточника» с «✓».
- **Переключатель языка KZ/RU** `{components.lang-switch}` — сегмент-пилюля; активный
  сегмент на `{colors.primary}` с белым label-caps; KZ — дефолт, RU — альтернатива.
  Интерактивная высота и ширина каждого сегмента ≥`{spacing.tap-target}` (44px),
  несмотря на 9.5px-кегль label-caps.
- **Переключатель темы** `{components.theme-switch}` — тумблер-пилюля (солнце/луна),
  трек `{colors.border}`, бегунок `{colors.surface}`; хитбокс ≥`{spacing.tap-target}`.
- **Фокус-ринг** `{components.focus-ring}` — обязателен на всех интерактивных целях,
  включая маркеры карты: кольцо `{colors.focus-ring}` (тёмная — `{colors.focus-ring-dark}`),
  ширина `{spacing.focus-ring-width}` (2px), офсет `{spacing.focus-ring-offset}` (2px),
  графический контраст ≥3:1 в обеих темах. Нативный браузерный ринг не используется.

## Do's and Don'ts

| Делать | Не делать |
|---|---|
| Флаг = нейтральный амбер `{colors.signal-attention}` «сигнал, требующий проверки» | **Никогда** не красить флаг тревожно-алым/красным «нарушение/виновен» |
| Статус маркера = ФОРМА/глиф (пустой / «!» / «✓» / двойное кольцо «выбран»); цвет вторичен | Кодировать статус маркера **только цветом** (амбер/золото/синий неразличимы для дальтоников, WCAG 1.4.1) |
| Амбер-флаг-маркер `{components.map-marker-flagged}` отличать глифом «требует проверки», равной визуальной весомости с голубым | Делать амбер-пин «громче» голубого — светофорная «тревога»/«горячая точка» до прочтения текста |
| Зелёный/красный «светофор» резервировать под будущий «светофор регионов» | Применять светофорный зелёный/красный к риск-флагам или состояниям карточки |
| Состояния нехватки данных — приглушённо-нейтральные, со сплошным бордюром и глифом | Штриховой бордюр / красный / «битый» вид у «нет данных» / «недостаточно» (читается как ошибка/дефект) |
| Честные состояния текстом `{colors.muted-text}`: «нет данных», «недостаточно сопоставимых данных», «объект не геопривязан» | Интерполировать, додумывать или прятать отсутствие данных |
| Кнопочная заливка — `{colors.primary-strong}` (AA для текста) | Ставить мелкий белый текст на `{colors.primary}` #1496C0 (только 3.4:1) |
| Ссылка-первоисточник = `{colors.link-on-sunken}`, проверена на surface И surface-sunken, всегда подчёркнута | Ставить `{colors.link}` на фон `{colors.surface-sunken}` (4.28:1 — ниже AA) |
| Выбранный фильтр-чип — насыщенная заливка + белый текст (6.53:1) + галочка | Выбранное состояние на `{colors.primary-soft}` (текст 3.95:1, фон-vs-невыбранный 1.08:1 — невидимо) |
| Видимый фокус-ринг `{components.focus-ring}` (≥3:1) на КАЖДОЙ интерактивной цели, включая маркеры | Полагаться на нативный браузерный ринг (теряется на голубоватом `{colors.surface-sunken}`) |
| Золото `{colors.accent}` — орнамент, выбранный маркер, акцент-линия | Использовать золото как семантику состояния или фон под мелкий текст |
| Орнамент қошқар мүйіз — тонкий линейный разделитель | Заливать орнаментом фоны рабочих экранов («фолк-фестиваль») |
| Шрифт с полным покрытием KZ-глифов (ә ғ қ ң ө ұ ү і); KZ — дефолт | Брать шрифт без расширенной казахской кириллицы (битые глифы на доменном тексте) |
| Контраст текста ≥ AA в обеих темах; **интерактивная** зона цели ≥44px (вкл. переключатели 9.5px) | Понижать контраст ради «воздуха»; путать визуальный размер с тап-целью |
| Сдержанные тени, иерархия тоном поверхностей | Тяжёлые «материал»-тени и градиентные фоны; тонкая граница как единственный носитель смысла |
| Карта плавная как 2GIS; UI-анимации функциональны, **уважают `prefers-reduced-motion`** | Декоративные анимации, отвлекающие от данных; движение без учёта `prefers-reduced-motion` |

---

> **Приоритет источника.** При любом конфликте между этим DESIGN.md и любым моком,
> импортом, скриншотом или сгенерированным ассетом — **побеждает DESIGN.md**.
>
> **Референсы (визуальная проба, не источник истины):**
> - Выбор основ: [`mockups/color-themes-1.html`](mockups/color-themes-1.html) — палитра (выбрана Тема 4 «Открытый город / Баланс») ·
>   [`mockups/directions-1.html`](mockups/directions-1.html) — личность (выбрано Направление B «Открытый город / Баланс»).
> - Ключевые экраны: [`mockups/key-map.html`](mockups/key-map.html) ·
>   [`mockups/key-contract-card.html`](mockups/key-contract-card.html) ·
>   [`mockups/key-flag-methodology.html`](mockups/key-flag-methodology.html) ·
>   [`mockups/key-district.html`](mockups/key-district.html) ·
>   [`mockups/key-contractor-card.html`](mockups/key-contractor-card.html) ·
>   [`mockups/key-search-filters.html`](mockups/key-search-filters.html) ·
>   [`mockups/key-telegram-bot.html`](mockups/key-telegram-bot.html).
