GOFERMART - сервис для подсчета баллов пользователя.
Техническое задание описано в файле [text](SPECIFICATION.md)
Подсчет баллов реализован суммой записей для пользователя в таблице t_account.
Поддерживаются флаги/системные переменные:
	-a/RUN_ADDRESS - address and port to run server (по умолчанию "localhost:8080")
	-d/DATABASE_URI - databse connect string (пример "postgres://manager:manager@localhost:5432/gofermart?sslmode=disable")
	-r/ACCRUAL_SYSTEM_ADDRESS - accrual system address (по умолчанию "localhost:8081")
	-s/JWT_SECRET - JWT secret (по умолчанию "superSecretKey", обязательно переопределить)
	-t/JWT_TTL - JWT TTL (по умолчанию 24 часа, пример заполнения "1h30m")
	-i/ORDER_CHECK_INTERVAL - interval for checking order status updates (по умолчанию 10с, пример заполнения "5s")
Для создания таблиц используются миграции goose (предварительно необходимо создать БД в postgresql).