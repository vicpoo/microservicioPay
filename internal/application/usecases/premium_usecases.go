//internal/application/usecases/premium_usecases.go
package usecases

import (
	"context"

	"github.com/kajve/payment-service/internal/domain/ports"
)

// --- Expirar premiums vencidos (llamado por el scheduler en main.go) ---

type ExpirarPremiumsUseCase struct {
	repo ports.OrderRepository
}

func NewExpirarPremiumsUseCase(repo ports.OrderRepository) *ExpirarPremiumsUseCase {
	return &ExpirarPremiumsUseCase{repo: repo}
}

func (uc *ExpirarPremiumsUseCase) Execute(ctx context.Context) (int, error) {
	return uc.repo.ExpirarPremiumsVencidos(ctx)
}

// --- Verificar si un usuario es premium ahora mismo ---

type VerificarPremiumUseCase struct {
	repo ports.OrderRepository
}

func NewVerificarPremiumUseCase(repo ports.OrderRepository) *VerificarPremiumUseCase {
	return &VerificarPremiumUseCase{repo: repo}
}

func (uc *VerificarPremiumUseCase) Execute(ctx context.Context, idUsuario int) (bool, error) {
	return uc.repo.EsPremium(ctx, idUsuario)
}