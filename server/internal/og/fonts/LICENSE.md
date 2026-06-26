# Шрифт DejaVu Sans

`DejaVuSans.ttf` — шрифт **DejaVu Sans** (вендорится для серверного рендера OG-картинки, Story 5.5),
покрывает кириллицу и казахские глифы (ә, ғ, қ, ң, ө, ұ, ү, һ, і) и знак тенге ₸.

Лицензия — пермиссивная (Bitstream Vera Fonts Copyright + Arev Fonts Copyright + public-domain
изменения проекта DejaVu): свободное использование, встраивание и распространение, в т.ч. в составе
ПО, без копилефта. Источник: https://dejavu-fonts.github.io/ . Полный текст — там же (License).

Встроен в бинарь через `//go:embed` (`server/internal/og/image.go`) — рантайм не зависит от
системных шрифтов.
