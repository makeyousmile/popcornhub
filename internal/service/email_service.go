package service

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"strings"
	"time"

	"popcornhub/internal/config"
)

type EmailService struct {
	cfg *config.Config
}

func NewEmailService(cfg *config.Config) *EmailService {
	return &EmailService{cfg: cfg}
}

func (s *EmailService) SendPasswordReset(toEmail, toName, resetURL string) error {
	subject := "Восстановление пароля в PopcornHub"
	greeting := "Здравствуйте!"
	if strings.TrimSpace(toName) != "" {
		greeting = fmt.Sprintf("Здравствуйте, %s!", strings.TrimSpace(toName))
	}

	body := fmt.Sprintf(
`%s

Вы запросили сброс пароля для вашей учётной записи в PopcornHub.

Для перехода к установке нового пароля нажмите на ссылку (действительна в течение 15 минут):
%s

Если вы не отправляли данный запрос, проигнорируйте это письмо — ваш пароль останется прежним.

--
С уважением,
Команда PopcornHub`, greeting, resetURL)

	slog.Info("Password reset requested",
		"recipient", toEmail,
		"reset_url", resetURL,
		"smtp_configured", s.cfg.SMTPHost != "",
	)

	// If SMTP is not configured, we log the link so the application remains fully functional in local/dev environments.
	if s.cfg.SMTPHost == "" {
		slog.Info("SMTP not configured: password reset email link logged to console",
			"recipient", toEmail,
			"reset_url", resetURL,
		)
		return nil
	}

	from := s.cfg.SMTPFrom
	if from == "" {
		from = "no-reply@popcornhub.local"
	}

	headers := make(map[string]string)
	headers["From"] = fmt.Sprintf("PopcornHub <%s>", from)
	headers["To"] = toEmail
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/plain; charset=UTF-8"
	headers["Date"] = time.Now().UTC().Format(time.RFC1123Z)

	var msg strings.Builder
	for k, v := range headers {
		msg.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	msg.WriteString("\r\n")
	msg.WriteString(body)

	addr := net.JoinHostPort(s.cfg.SMTPHost, s.cfg.SMTPPort)

	// Port 465 typically uses implicit TLS (SMTPS)
	if s.cfg.SMTPPort == "465" {
		tlsConfig := &tls.Config{
			ServerName: s.cfg.SMTPHost,
		}
		conn, err := tls.Dial("tcp", addr, tlsConfig)
		if err != nil {
			slog.Error("Failed to connect to SMTP server over TLS", "addr", addr, "error", err)
			return err
		}
		defer conn.Close()

		client, err := smtp.NewClient(conn, s.cfg.SMTPHost)
		if err != nil {
			slog.Error("Failed to create SMTP client", "error", err)
			return err
		}
		defer client.Close()

		if s.cfg.SMTPUser != "" && s.cfg.SMTPPassword != "" {
			auth := smtp.PlainAuth("", s.cfg.SMTPUser, s.cfg.SMTPPassword, s.cfg.SMTPHost)
			if err := client.Auth(auth); err != nil {
				slog.Error("SMTP authentication failed", "error", err)
				return err
			}
		}

		if err := client.Mail(from); err != nil {
			return err
		}
		if err := client.Rcpt(toEmail); err != nil {
			return err
		}
		w, err := client.Data()
		if err != nil {
			return err
		}
		if _, err := w.Write([]byte(msg.String())); err != nil {
			return err
		}
		if err := w.Close(); err != nil {
			return err
		}
		return client.Quit()
	}

	// Standard STARTTLS (e.g. port 587 or 25)
	var auth smtp.Auth
	if s.cfg.SMTPUser != "" && s.cfg.SMTPPassword != "" {
		auth = smtp.PlainAuth("", s.cfg.SMTPUser, s.cfg.SMTPPassword, s.cfg.SMTPHost)
	}

	if err := smtp.SendMail(addr, auth, from, []string{toEmail}, []byte(msg.String())); err != nil {
		slog.Error("Failed to send SMTP email", "recipient", toEmail, "error", err)
		return err
	}

	slog.Info("Password reset email sent via SMTP successfully", "recipient", toEmail)
	return nil
}
