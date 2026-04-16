# go-musthave-shortener-tpl

Шаблон репозитория для трека «Сервис сокращения URL».

## Начало работы

1. Склонируйте репозиторий в любую подходящую директорию на вашем компьютере.
2. В корне репозитория выполните команду `go mod init <name>` (где `<name>` — адрес вашего репозитория на GitHub без префикса `https://`) для создания модуля.

## Обновление шаблона

Чтобы иметь возможность получать обновления автотестов и других частей шаблона, выполните команду:

```
git remote add -m v2 template https://github.com/Yandex-Practicum/go-musthave-shortener-tpl.git
```

Для обновления кода автотестов выполните команду:

```
git fetch template && git checkout template/v2 .github
```

Затем добавьте полученные изменения в свой репозиторий.

## Запуск автотестов

Для успешного запуска автотестов называйте ветки `iter<number>`, где `<number>` — порядковый номер инкремента. Например, в ветке с названием `iter4` запустятся автотесты для инкрементов с первого по четвёртый.

При мёрже ветки с инкрементом в основную ветку `main` будут запускаться все автотесты.

Подробнее про локальный и автоматический запуск читайте в [README автотестов](https://github.com/Yandex-Practicum/go-autotests).

## Структура проекта

Приведённая в этом репозитории структура проекта является рекомендуемой, но не обязательной.

Это лишь пример организации кода, который поможет вам в реализации сервиса.

При необходимости можно вносить изменения в структуру проекта, использовать любые библиотеки и предпочитаемые структурные паттерны организации кода приложения, например:

- **DDD** (Domain-Driven Design)
- **Clean Architecture**
- **Hexagonal Architecture**
- **Layered Architecture**

## Бенчмарки

1. до / после:

### Анализ базового профиля

По `go tool pprof -top profiles/base.pprof` выделялись: крупные аллокации вокруг `bufio.NewWriterSize` из-за преждевременного создания `gzip.Writer` при каждом запросе (даже когда ответ `text/plain` и сжатие не включается), `encoding/hex.EncodeToString` и `hmac.New` в auth-middleware на каждый запрос, лишняя аллокация слайса в `generateShortID`, пересоздание `map` в `processDeletes` на каждом тике.

### Внесённые оптимизации

- **gzip**: `gzip.Writer` создаётся только если ответ реально уходит с `Content-Encoding: gzip`.
- **auth**: один раз `[]byte` для секретного ключа, пул `hash.Hash` для HMAC, шестнадцатеричная подпись и user id без лишних `EncodeToString` / `strings.Split`, разбор cookie через `IndexByte`.
- **URLService**: генерация short id в массиве на стеке; `clear(userBatches)` вместо новой `map` на каждом тике.
- **handler**: ответ text/plain через `io.WriteString`.
- **repository**: начальная ёмкость слайса в `GetByUserID`.

### Вывод `pprof -top -diff_base` (base.pprof → result.pprof)

```text
File: meow-shortener
Type: inuse_space
Time: 2026-04-01 09:24:05 MSK
Showing nodes accounting for -256.80kB, 5.55% of 4626.59kB total
Dropped 2 nodes (cum <= 23.13kB)
      flat  flat%   sum%        cum   cum%
  768.26kB 16.61% 16.61%   768.26kB 16.61%  go.uber.org/zap/zapcore.newCounters (inline)
    -514kB 11.11%  5.50%     -514kB 11.11%  bufio.NewWriterSize (inline)
     513kB 11.09% 16.58%      513kB 11.09%  runtime.allocm
 -512.04kB 11.07%  5.52%  -512.02kB 11.07%  github.com/mmeow0/meow-shortener/internal/handler.(*URLHandler).CreateShortURLPlain
 -512.02kB 11.07%  5.55%  -512.02kB 11.07%  github.com/google/uuid.UUID.String (inline)
  512.02kB 11.07%  5.52%   512.02kB 11.07%  unicode.map.init.0
 -512.02kB 11.07%  5.55%  -512.02kB 11.07%  encoding/hex.EncodeToString (inline)
         0     0%  5.55% -1024.04kB 22.13%  github.com/go-chi/chi/v5.(*Mux).ServeHTTP
         0     0%  5.55%  -512.02kB 11.07%  github.com/go-chi/chi/v5.(*Mux).routeHTTP
         0     0%  5.55%   768.26kB 16.61%  github.com/mmeow0/meow-shortener/internal/app.InitializeApp
         0     0%  5.55%   768.26kB 16.61%  github.com/mmeow0/meow-shortener/internal/logger.NewLogger
         0     0%  5.55% -1024.04kB 22.13%  github.com/mmeow0/meow-shortener/internal/middleware.GzipMiddleware.func1
         0     0%  5.55%  -512.02kB 11.07%  github.com/mmeow0/meow-shortener/internal/middleware.generateUserID
         0     0%  5.55% -2573.53kB 55.62%  github.com/mmeow0/meow-shortener/internal/router.NewRouter.AuthMiddleware.func2.1
         0     0%  5.55%  1549.49kB 33.49%  github.com/mmeow0/meow-shortener/internal/router.NewRouter.AuthMiddleware.func3.1
         0     0%  5.55% -1024.04kB 22.13%  github.com/mmeow0/meow-shortener/internal/router.NewRouter.RequestLogger.func1.1
         0     0%  5.55%  -512.02kB 11.07%  github.com/mmeow0/meow-shortener/internal/service.(*URLService).generateUUID
         0     0%  5.55%   768.26kB 16.61%  go.uber.org/zap.(*Logger).WithOptions
         0     0%  5.55%   768.26kB 16.61%  go.uber.org/zap.Config.Build
         0     0%  5.55%   768.26kB 16.61%  go.uber.org/zap.Config.buildOptions.WrapCore.func5
         0     0%  5.55%   768.26kB 16.61%  go.uber.org/zap.Config.buildOptions.func1
         0     0%  5.55%   768.26kB 16.61%  go.uber.org/zap.New
         0     0%  5.55%   768.26kB 16.61%  go.uber.org/zap.optionFunc.apply
         0     0%  5.55%   768.26kB 16.61%  go.uber.org/zap/zapcore.NewSamplerWithOptions
         0     0%  5.55%   768.26kB 16.61%  main.main
         0     0%  5.55% -1538.04kB 33.24%  net/http.(*conn).serve
         0     0%  5.55% -1024.04kB 22.13%  net/http.HandlerFunc.ServeHTTP
         0     0%  5.55%     -514kB 11.11%  net/http.newBufioWriterSize
         0     0%  5.55% -1024.04kB 22.13%  net/http.serverHandler.ServeHTTP
         0     0%  5.55%   512.02kB 11.07%  runtime.doInit (inline)
         0     0%  5.55%   512.02kB 11.07%  runtime.doInit1
         0     0%  5.55%  1280.28kB 27.67%  runtime.main
         0     0%  5.55%      513kB 11.09%  runtime.mstart
         0     0%  5.55%      513kB 11.09%  runtime.mstart0
         0     0%  5.55%      513kB 11.09%  runtime.mstart1
         0     0%  5.55%      513kB 11.09%  runtime.newm
         0     0%  5.55%      513kB 11.09%  runtime.resetspinning
         0     0%  5.55%      513kB 11.09%  runtime.schedule
         0     0%  5.55%      513kB 11.09%  runtime.startm
         0     0%  5.55%      513kB 11.09%  runtime.wakep
         0     0%  5.55%   512.02kB 11.07%  unicode.init
```
