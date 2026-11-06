package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/term"

	"smetchic/application"
	"smetchic/model"
	"smetchic/storage/postgres"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) != 1 {
		return fmt.Errorf("использование: go run ./cmd/createuser (все значения вводятся интерактивно)")
	}
	reader := bufio.NewReader(os.Stdin)
	email, err := prompt(reader, "Email: ")
	if err != nil {
		return err
	}
	name, err := prompt(reader, "Имя: ")
	if err != nil {
		return err
	}
	companyID, err := prompt(reader, "Company ID: ")
	if err != nil {
		return err
	}
	roleText, err := prompt(reader, "Роль [owner/admin/manager/operator] (по умолчанию owner): ")
	if err != nil {
		return err
	}
	if roleText == "" {
		roleText = string(model.CompanyRoleOwner)
	}
	role := model.CompanyRole(roleText)

	password, err := readPassword(reader, "Пароль (не менее 12 байт): ")
	if err != nil {
		return err
	}
	confirmation, err := readPassword(reader, "Повторите пароль: ")
	if err != nil {
		return err
	}
	if password != confirmation {
		return fmt.Errorf("пароли не совпадают")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := postgres.OpenFromEnv(ctx)
	if err != nil {
		return fmt.Errorf("подключение к PostgreSQL: %w", err)
	}
	defer db.Close()

	repository := postgres.NewLifecycleRepository(db)
	if _, err := repository.GetCompany(ctx, strings.TrimSpace(companyID)); err != nil {
		return fmt.Errorf("компания %q не найдена: %w", strings.TrimSpace(companyID), err)
	}
	user, err := application.NewAuthenticationService(repository).CreateUser(
		ctx, email, name, password, strings.TrimSpace(companyID), role,
	)
	if err != nil {
		return fmt.Errorf("не удалось создать пользователя: %w", err)
	}
	fmt.Printf("Пользователь %s создан и добавлен в компанию %s с ролью %s.\n", user.Email, strings.TrimSpace(companyID), role)
	return nil
}

func prompt(reader *bufio.Reader, label string) (string, error) {
	fmt.Fprint(os.Stdout, label)
	value, err := reader.ReadString('\n')
	if err != nil && len(value) == 0 {
		return "", fmt.Errorf("не удалось прочитать ввод: %w", err)
	}
	return strings.TrimSpace(value), nil
}

func readPassword(reader *bufio.Reader, promptText string) (string, error) {
	fmt.Fprint(os.Stdout, promptText)
	if term.IsTerminal(int(os.Stdin.Fd())) {
		password, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stdout)
		if err != nil {
			return "", fmt.Errorf("не удалось прочитать пароль: %w", err)
		}
		return string(password), nil
	}
	password, err := reader.ReadString('\n')
	if err != nil && len(password) == 0 {
		return "", fmt.Errorf("не удалось прочитать пароль: %w", err)
	}
	return strings.TrimRight(password, "\r\n"), nil
}
