package domain

import "time"

type Shop struct {
	ID                       string
	ShopDomain               string
	ShopifyAccessToken       string
	WhatsAppAccessToken      string
	WhatsAppPhoneNumberID    string
	WhatsAppTemplateName     string
	WhatsAppTemplateLanguage string
	SetupToken               string
	WebhookID                int64
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

type ShopWhatsAppConfig struct {
	AccessToken      string
	PhoneNumberID    string
	TemplateName     string
	TemplateLanguage string
}
