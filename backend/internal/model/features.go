package model

import "github.com/sklyar-vlad/HackAeza/backend/internal/entity"

func Features() []entity.FeatureDefinition {
	return []entity.FeatureDefinition{
		{Key: entity.FeatureRenewalWithoutAutoProlong, Name: "Близкое окончание без автопродления",
			Description: "Есть ближайшая оплата на горизонте прогноза при выключенном автопродлении.", DefaultCoefficient: 0.4,
			RequiredFields:    []string{"services.expiresAt", "services.autoProlong", "services.status"},
			MissingDataPolicy: "Признак неприменим, если нет данных об услугах."},
		{Key: entity.FeatureInsufficientRenewalFunds, Name: "Недостаток средств",
			Description: "Совместная обеспеченность ближайших продлений доступным балансом аккаунта.", DefaultCoefficient: 0.35,
			RequiredFields:    []string{"accounts.balance", "accounts.bonusBalance", "accounts.bonusState", "services.price"},
			MissingDataPolicy: "Признак неприменим без баланса или стоимости ближайших продлений."},
		{Key: entity.FeaturePaymentWaiting, Name: "Ожидание оплаты",
			Description: "Наличие prolong_wait; длительность ожидания и дефицит средств могут усиливать признак.", DefaultCoefficient: 0.25,
			RequiredFields:    []string{"services.status", "services.expiresAt"},
			MissingDataPolicy: "Признак неприменим без статуса услуги."},
	}
}
