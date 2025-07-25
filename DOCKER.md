# Docker Руководство для William Bot

Это руководство описывает как запустить William Bot локально с помощью Docker и docker-compose.

## Быстрый старт

### 1. Подготовка

```bash
# Клонируйте репозиторий
git clone <repository-url>
cd william

# Создайте файл с переменными окружения
cp docker-compose.env.example .env

# Отредактируйте .env файл с вашими API ключами
nano .env
```

### 2. Запуск

```bash
# Запустите development среду (с автоматическим созданием .env)
make docker-compose-dev

# ИЛИ запустите сервисы в фоне
make docker-compose-up

# Запустите миграции базы данных
make docker-compose-migrate

# Проверьте логи
make docker-compose-logs
```

### 3. Остановка

```bash
# Остановите сервисы
make docker-compose-down

# Полная очистка (включая volumes)
make docker-compose-clean
```

## Структура сервисов

### PostgreSQL (`postgres`)
- **Образ**: `postgres:15-alpine`
- **Порт**: `5432`
- **База данных**: `william`
- **Пользователь**: `william`
- **Пароль**: `william_password`
- **Volume**: `postgres_data` для постоянного хранения

### William Bot (`william`)
- **Образ**: Собирается из локального Dockerfile
- **Зависимости**: PostgreSQL должен быть healthy
- **Конфигурация**: Загружается из `/app/config/app.toml`
- **Environment**: Переменные из `.env` файла

### Migration Service (`migrate`)
- **Образ**: Тот же что и william
- **Назначение**: Одноразовый запуск миграций
- **Команда**: `goose -dir /app/migrations postgres "$$PG_DSN" up`

## Переменные окружения

### Обязательные
```bash
TG_BOT_TOKEN=your_telegram_bot_token_here
OPENAI_API_KEY=your_openai_api_key_here
```

### Опциональные (с defaults)
```bash
OPENAI_MODEL=gpt-4o-mini
MAX_MSG_BUFFER=100
CTX_MAX_TOKENS=2048
TZ=Europe/Belgrade
```

## Полезные команды

### Логи и отладка
```bash
# Логи всех сервисов
docker-compose logs

# Логи конкретного сервиса
docker-compose logs william
docker-compose logs postgres

# Логи в реальном времени
docker-compose logs -f william

# Выполнить команду в контейнере
docker-compose exec william /bin/sh
docker-compose exec postgres psql -U william -d william
```

### База данных
```bash
# Подключиться к PostgreSQL
docker-compose exec postgres psql -U william -d william

# Запустить миграции
docker-compose run --rm migrate

# Создать backup
docker-compose exec postgres pg_dump -U william william > backup.sql

# Восстановить backup
docker-compose exec -T postgres psql -U william william < backup.sql
```

### Разработка
```bash
# Пересобрать сервис после изменений кода
docker-compose build william
docker-compose up -d william

# Перезапустить только william
docker-compose restart william

# Отслеживать изменения конфигурации (volume mounted)
# config/app.toml автоматически обновляется в контейнере
```

## Решение проблем

### William не запускается
```bash
# Проверьте логи
docker-compose logs william

# Проверьте переменные окружения
docker-compose config

# Проверьте что БД доступна
docker-compose exec william ping postgres
```

### Проблемы с PostgreSQL
```bash
# Проверьте статус
docker-compose ps postgres

# Проверьте логи
docker-compose logs postgres

# Проверьте подключение
docker-compose exec postgres pg_isready -U william
```

### Проблемы с миграциями
```bash
# Проверьте статус миграций
docker-compose exec postgres psql -U william -d william -c "SELECT * FROM goose_db_version;"

# Запустите миграции вручную
docker-compose run --rm migrate
```

### Очистка и перезапуск
```bash
# Остановить все и удалить volumes
docker-compose down -v

# Удалить образы
docker-compose down --rmi all

# Полная очистка
make docker-compose-clean
docker system prune -a
```

## Production настройки

Для production развертывания рекомендуется:

1. **Использовать external database**
   ```yaml
   # Закомментировать postgres service
   # Использовать внешний PG_DSN
   ```

2. **Настроить secrets management**
   ```bash
   # Использовать Docker secrets или внешние системы
   # Не хранить API ключи в .env файлах
   ```

3. **Настроить мониторинг**
   ```yaml
   # Добавить healthchecks
   # Настроить logging driver
   # Добавить metrics сбор
   ```

4. **Оптимизировать ресources**
   ```yaml
   # Добавить resource limits
   # Настроить restart policies
   # Использовать multi-stage builds
   ```

## Разработка с hot reload

Для разработки с автоматической перезагрузкой:

1. **Используйте volume mounting**
   ```yaml
   # В docker-compose.override.yml уже настроено
   volumes:
     - ./config:/app/config:ro
   ```

2. **Настройте air для Go hot reload**
   ```bash
   # Установите air
   go install github.com/cosmtrek/air@latest
   
   # Запустите в отдельном терминале
   air
   ```

## Безопасность

⚠️ **Важные рекомендации:**

1. **Никогда не commit .env файлы**
2. **Используйте сложные пароли для production**
3. **Ограничьте сетевой доступ к PostgreSQL**
4. **Регулярно обновляйте базовые образы**
5. **Сканируйте образы на уязвимости** 