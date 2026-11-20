# mine_go_auth

Микросервис авторизации Alpha на базе существующего проекта `mine_go_auth`: Go 1.26, Fiber v2, PostgreSQL и goose. Сайт и лаунчер работают с общей таблицей аккаунтов. Для запуска Minecraft сервис выдаёт отдельный токен, привязанный к игровому профилю и серверу.

## Запуск

1. Укажите параметры подключения к PostgreSQL. При старте приложение применяет встроенные миграции goose под блокировкой PostgreSQL. Оригинальные таблицы и записи сохраняются. Пользователи старых сессий входят повторно.
2. Создайте постоянный RSA-ключ и предоставьте сервису права на чтение. Используйте один и тот же защищённый ключ на всех экземплярах сервиса. Подпись SHA1withRSA используется только для поля `textures` по протоколу Yggdrasil.

   ```sh
   mkdir -p secrets
   chmod 700 secrets
   (umask 077 && openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out secrets/yggdrasil.pem)
   ```

3. Запустите `go run ./cmd` с окружением из таблицы. Для Compose скопируйте `.env.example` в `.env`, задайте свою конфигурацию и выполните `docker compose up --build`. Compose публикует порт сервиса на loopback, а ключ монтирует для чтения.
4. В production опубликуйте API по HTTPS. Для сайта проксируйте `/api/auth` через origin сайта, чтобы браузер использовал cookies для запросов того же origin. На доверенном прокси перезаписывайте `X-Real-IP` адресом клиента и задайте в приложении адрес этого прокси. Ограничьте доступ к `/internal/*` сетью сервиса управления серверами.
5. Для лаунчера задайте HTTPS-origin **без пути** при сборке. В debug для локальных тестов также допустимы `http://127.0.0.1:PORT` и `http://[::1]:PORT`.

   ```sh
   LAUNCHER_AUTH_URL=https://auth.example.com scripts/with-local-tools.sh npm run desktop:build
   ```

Запускайте команду сборки из корня лаунчера. Provisioning и развёртывание игровых серверов выполняются отдельно. У сервиса нет SMTP. Поле `email_verified_at` зарезервировано для подтверждения email. При добавлении подтверждения или восстановления храните хеши одноразовых токенов со сроком действия и однократным использованием.

| Переменная                                                              | Назначение                                                                                                    |
| ----------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------- |
| `APP_HOST`, `APP_PORT`                                                  | Адрес и порт прослушивания. По умолчанию `0.0.0.0:3000`                                                       |
| `APP_ENV`                                                               | Режим журналирования. Для эксплуатации используйте `production`                                               |
| `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_SSLMODE` | Подключение PostgreSQL. Для удалённой БД включите проверку сертификата                                        |
| `AUTH_SIGNING_KEY_FILE`                                                 | Обязательный путь к RSA-ключу PKCS#1 или PKCS#8 PEM                                                           |
| `AUTH_SIGNING_KEY_HOST_FILE`                                            | Путь к RSA-ключу на хосте при использовании Compose. По умолчанию `./secrets/yggdrasil.pem`                   |
| `AUTH_PROVISION_TOKEN`                                                  | Случайный секрет из 32–512 символов без пробелов. При пустом значении provisioning отключён                   |
| `AUTH_SKIN_DOMAINS`                                                     | Список точных HTTPS-хостов для скинов через запятую. Пустой список отключает пользовательские ссылки на скины |
| `AUTH_TRUSTED_PROXIES`                                                  | IP или CIDR доверенных прокси через запятую. При пустом значении используется адрес TCP-соединения            |
| `LAUNCHER_AUTH_URL`                                                     | Origin API, фиксируемый при сборке лаунчера                                                                   |

## API аккаунтов

Для запросов с телом и браузерных POST на refresh и logout используйте `Content-Type: application/json`. Браузер получает cookies `__Host-alpha_access` и `__Host-alpha_refresh` с `Secure`, `HttpOnly` и `SameSite=Strict`. Лаунчер посылает `client: "launcher"` в login, refresh и logout, передаёт access в `Authorization: Bearer …`, а refresh в поле `refreshToken`. В JSON-ответе лаунчера поле `sessionId` остаётся прежним после ротации. Сам refresh меняется при каждом успешном обновлении. Лаунчер хранит пароль только на время запроса и по согласию пользователя записывает refresh в системное хранилище.

| Метод и путь                            | Тело или действие                                                                                         |
| --------------------------------------- | --------------------------------------------------------------------------------------------------------- |
| `POST /api/auth/register`               | `nickname`, `password`, `invitation_code`, необязательный `email`. Ответ `201` с `user`                   |
| `POST /api/auth/login`                  | `login` (логин или email) и `password`. Ответ `200` с `user` и, для лаунчера, токенами                    |
| `POST /api/auth/refresh`                | Ротирует access и refresh. Ответ `200` с парой для лаунчера, `204` с cookies для сайта                    |
| `POST /api/auth/logout`                 | Отзывает текущую сессию и выданные через неё игровые токены. Ответ `204`                                  |
| `POST /api/auth/logout-all`             | Отзывает все пользовательские сессии и связанные игровые токены. Требует access. Ответ `204`              |
| `GET /api/auth/profile`                 | Возвращает `user`. Требует access                                                                         |
| `GET /api/auth/game-profiles`           | Возвращает `profiles` вошедшего пользователя. Требует access                                              |
| `POST /api/auth/game-token`             | Принимает принадлежащий аккаунту `profileId`. Возвращает `201`, `accessToken`, `profile` и `injectorPath` |
| `PUT /api/auth/game-profiles/{id}/skin` | Принимает `url` и `slim`. Пустой `url` сбрасывает скин. Требует access владельца                          |

Access действует 10 минут, refresh — до 30 дней с момента входа, игровой токен — 8 часов, запись Join — 30 секунд. Endpoint refresh отвергает повторное использование старого refresh и отзывает его сессию. Число активных пользовательских сессий и игровых токенов на аккаунт, а также число ротаций одной сессии ограничены. Лимит проверки пароля общий для всех имён входа одного аккаунта, включая игровые профили. Лимиты и сроки жизни находятся в `constants/auth_constants.go`.

Регистрация использует существующие таблицы приглашений. Код выдаёт администратор через действующий процесс управления приглашениями. Для одноразовой локальной проверки владелец тестовой БД может вставить код в `invitations` с `created_by` от тестового аккаунта. Регистрация атомарно связывает приглашение с созданным аккаунтом.

На чистой БД администратор может создать выключенного владельца приглашений и сгенерировать первый одноразовый код из `psql` после миграции. Результат запроса передайте получателю приватно.

```sql
BEGIN;
INSERT INTO users (nickname, password_hash, is_active)
VALUES ('_invite_issuer', '!', false)
ON CONFLICT (nickname) DO NOTHING;
INSERT INTO invitations (code, created_by, expires_at)
SELECT left(replace(gen_random_uuid()::text, '-', ''), 20), id, now() + interval '1 day'
FROM users
WHERE nickname = '_invite_issuer' AND NOT is_active
RETURNING code;
COMMIT;
```

## Provisioning игрового профиля

При создании сервера система управления вызывает `POST /internal/game-profiles` с сервисным Bearer-токеном и JSON `{"userId":"<uuid>","realm":"survival","nickname":"Player_Name"}`. Вызывайте для каждого аккаунта, которому нужен профиль на сервере. Повторный запрос с тем же аккаунтом и `realm` возвращает существующий UUID и ник. Ник уникален без учёта регистра в пределах `realm`. Храните скин как URL на контролируемом HTTPS-хосте. Путь оканчивается SHA-256 содержимого в нижнем регистре с необязательным расширением `.png`, без query и fragment. При изменении изображения публикуйте его под новым хешем, который Minecraft использует для кэша. Сервис хранит метаданные скина, а объектное хранилище или CDN отвечает за содержимое.

## Yggdrasil и Authlib Injector

Лаунчер и сервер Minecraft используют один и тот же корень `https://auth.example.com/yggdrasil/<realm>/`. Лаунчер принимает `injectorPath` только для выбранного профиля и соединяет его с origin сборки. Перед запуском он проверяет SHA-256 закреплённого Authlib Injector 1.2.8 и передаёт Minecraft только игровой токен.

| Метод | Путь относительно корня                                                                                                    |
| ----- | -------------------------------------------------------------------------------------------------------------------------- |
| GET   | `/` — метаданные сервера и открытый ключ текстур                                                                           |
| POST  | `/authserver/authenticate`, `/authserver/refresh`, `/authserver/validate`, `/authserver/invalidate`, `/authserver/signout` |
| POST  | `/sessionserver/session/minecraft/join`                                                                                    |
| GET   | `/sessionserver/session/minecraft/hasJoined`                                                                               |
| GET   | `/sessionserver/session/minecraft/profile/{uuid}?unsigned=false` — профиль и подписанная текстура                          |
| POST  | `/api/profiles/minecraft` — поиск профилей по никам внутри realm                                                           |

Yggdrasil Authenticate и Signout принимают email или ник игрового профиля в выбранном realm. Для API аккаунтов доступны email и логин аккаунта.

На Minecraft-сервере добавьте `-javaagent:/path/to/authlib-injector-1.2.8.jar=https://auth.example.com/yggdrasil/<realm>/` до `-jar`. В `server.properties` оставьте `online-mode=true` и `enforce-secure-profile=true`. По документации Injector 1.2.0+ снимает требование Mojang-ключа профиля для серверов с собственной авторизацией. Сервис объявляет `feature.enable_profile_key=false`; выпуск ключей подписанного чата за пределами данной реализации. Настройки прокси Minecraft и firewall должны исключать прямой вход в offline-mode.

[Спецификация Yggdrasil](https://yushijinhun.github.io/authlib-injector/en/yggdrasil-server-technical-specification.html) · [Настройки Minecraft-сервера](https://yushijinhun.github.io/authlib-injector/en/using-authlib-injector-on-a-minecraft-server.html)

## Проверка

```sh
go test -race -v ./...
go vet ./...
go mod verify
```

Задайте `MINECRAFT_AUTH_TEST_DATABASE` для тестов с одноразовой БД. Тесты создают временные схемы, проверяют чистую миграцию и обновление схемы архива, cookies и Bearer, конкурентную ротацию и отзыв, выдачу игрового токена, разделение серверов, Join / HasJoined и профиль с подписью. С `MINECRAFT_AUTH_SMOKE_JAVA`, `MINECRAFT_AUTH_SMOKE_AGENT` и `MINECRAFT_AUTH_SMOKE_CLASSPATH` они дополнительно запускают Java 21 с закреплённым Injector и библиотеками Minecraft 1.21.1. Smoke проходит против HTTP-фикстуры и реальной PostgreSQL-реализации. Для проверки Rust-клиента подготовьте отдельный экземпляр API, тестовый аккаунт `SmokeAccount` с паролем из теста, профили `survival` и `creative` и свободное приглашение `SMOKEREG`. Задайте `MINECRAFT_AUTH_SMOKE_ORIGIN` и запустите из корня лаунчера.

```sh
scripts/with-local-tools.sh cargo test -p launcher-core --locked --test auth platform_lifecycle_against_mine_go_auth -- --ignored
```

Для запуска Minecraft с тем же API используйте пример `launch_smoke` с `--features dev-auth` и путём к изолированному корню установки игры. При заданном `MINECRAFT_AUTH_SMOKE_ORIGIN` пример читает пароль тестового аккаунта из stdin и запускает платформенный сценарий.

Для локального HTTPS-стенда передайте PEM тестового CA в `MINECRAFT_AUTH_SMOKE_CA`. Этот режим доступен в debug с `dev-auth` и принимает только HTTPS на loopback. JVM клиента и сервера можно передать отдельную копию truststore с тем же CA. Для `launch_smoke` путь к тестовой обёртке Java задаётся через `MINECRAFT_AUTH_SMOKE_JAVA_WRAPPER`.

Положительный сценарий выбирает `survival`. С `MINECRAFT_AUTH_SMOKE_REALM=creative` клиент получает профиль другого realm. С `MINECRAFT_AUTH_SMOKE_REVOKE=1` пример отзывает сессию сразу после выдачи игрового токена, чтобы проверить отказ при подключении. Используйте эти флаги с тестовым аккаунтом на изолированном стенде.

20 ноября 2026 на macOS ARM64 прошли тесты PostgreSQL 16.16, два JVM smoke с Authlib Injector 1.2.8 и Java 21.0.7, тест Rust-клиента на запущенном сервисе и запуск клиента Minecraft 1.21.1 с тем же API. Журнал игры подтвердил активный агент и инициализацию рендера. Полный Go race-прогон выполнен на Go 1.27.2. Те же интеграционные пакеты прошли на Go 1.26.0 с временным module-файлом. В репозитории сохранён исходный минимум Go 1.26.1. Проверки фронтенда и Rust, включая ACL и compile-time запрет mock в release, также прошли.

В отдельном сквозном прогоне на macOS ARM64 клиент из `launch_smoke` вошёл на vanilla Minecraft 1.21.1 с `online-mode=true` и `enforce-secure-profile=true`. Rust, клиент и сервер использовали один локальный HTTPS-origin с тестовым CA. Сервер подтвердил ник и UUID, клиент получил PNG с SHA-256 из подписанного профиля, а тестовый мир сохранился при остановке сервера.

| Профиль и состояние токена       | Join              | HasJoined на `survival` | Результат            |
| -------------------------------- | ----------------- | ----------------------- | -------------------- |
| `survival`, действительный       | 204               | 200                     | Игрок вошёл          |
| `creative`, действительный       | 204 на `creative` | 204                     | Сервер отклонил вход |
| `survival`, отозван после выдачи | 403               | —                       | Вход отклонён        |

Тест проверял локальную конфигурацию. Публичный HTTPS-origin и релизную сборку можно проверить на стенде развёртывания.

Спецификация проекта сохраняет Fiber v2. Эта ветка снята с поддержки сопровождением Fiber. [Политика поддержки Fiber](https://github.com/gofiber/fiber/security)
