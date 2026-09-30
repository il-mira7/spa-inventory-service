package service

import (
	"context"
	"fmt"
	"time"

	"github.com/il-mira7/spa-inventory-service/internal/calc"
	"github.com/il-mira7/spa-inventory-service/internal/domain"
	"github.com/il-mira7/spa-inventory-service/internal/repository/postgres"
	"github.com/shopspring/decimal"
)

// CreateMovementCommand содержит параметры для проведения складской операции
type CreateMovementCommand struct {
	DocumentNo    string
	OperationDate time.Time
	SKU           string
	LocationID    string
	OperationType domain.OperationType
	Quantity      decimal.Decimal
	BatchID       string
	ExpiryDate    *time.Time
	PurchasePrice decimal.Decimal
	InvoiceNo     string
	Note          string
}

// MovementService определяет бизнес-контракт операций со складом и журналом проводок
type MovementService interface {
	ProcessMovement(ctx context.Context, cmd CreateMovementCommand) (*domain.MovementResult, error)
	GetMovements(ctx context.Context, filter postgres.MovementFilter, limit, offset int) ([]domain.Movement, int64, error)
}

type movementService struct {
	txManager    postgres.TxManager
	movementRepo postgres.MovementRepository
	stockRepo    postgres.StockRepository
	productRepo  postgres.ProductRepository
}

// NewMovementService создает новый экземпляр сервиса складских операций
func NewMovementService(
	txManager postgres.TxManager,
	movementRepo postgres.MovementRepository,
	stockRepo postgres.StockRepository,
	productRepo postgres.ProductRepository,
) MovementService {
	return &movementService{
		txManager:    txManager,
		movementRepo: movementRepo,
		stockRepo:    stockRepo,
		productRepo:  productRepo,
	}
}

// ProcessMovement маршрутизирует операцию на специализированный обработчик в зависимости от типа
func (s *movementService) ProcessMovement(ctx context.Context, cmd CreateMovementCommand) (*domain.MovementResult, error) {
	if err := s.validateCommand(ctx, cmd); err != nil {
		return nil, err
	}

	switch cmd.OperationType {
	case domain.OperationReceipt:
		return s.processReceipt(ctx, cmd)
	case domain.OperationConsume:
		return s.processConsumption(ctx, cmd)
	case domain.OperationWriteoff:
		return s.processWriteoff(ctx, cmd)
	case domain.OperationReturn:
		return s.processReturn(ctx, cmd)
	case domain.OperationCorrection:
		return s.processCorrection(ctx, cmd)
	default:
		return nil, domain.ErrInvalidOperation
	}
}

// GetMovements возвращает отфильтрованный список движений с пагинацией
func (s *movementService) GetMovements(ctx context.Context, filter postgres.MovementFilter, limit, offset int) ([]domain.Movement, int64, error) {
	return s.movementRepo.GetMovements(ctx, filter, limit, offset)
}

func (s *movementService) validateCommand(ctx context.Context, cmd CreateMovementCommand) error {
	if cmd.DocumentNo == "" {
		return domain.ErrInvalidDocumentNo
	}
	if !cmd.OperationType.IsValid() {
		return domain.ErrInvalidOperation
	}
	if cmd.Quantity.LessThanOrEqual(decimal.Zero) && cmd.OperationType != domain.OperationCorrection {
		return domain.ErrInvalidQuantity
	}
	if cmd.OperationDate.After(time.Now().Add(1 * time.Minute)) {
		return domain.ErrFutureDate
	}

	if _, err := s.productRepo.GetProductBySKU(ctx, cmd.SKU); err != nil {
		return err
	}

	exists, err := s.productRepo.LocationExists(ctx, cmd.LocationID)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("location %s not found: %w", cmd.LocationID, domain.ErrNotFound)
	}

	return nil
}

func (s *movementService) processReceipt(ctx context.Context, cmd CreateMovementCommand) (*domain.MovementResult, error) {
	var result *domain.MovementResult
	err := s.txManager.RunInTx(ctx, func(txCtx context.Context) error {
		if err := s.recordProcessedDocument(txCtx, cmd); err != nil {
			return err
		}
		if err := s.upsertReceiptBatch(txCtx, cmd); err != nil {
			return err
		}
		res, err := s.insertMovementsAndBuildResult(txCtx, cmd, []domain.Movement{s.buildBaseMovement(cmd)})
		if err != nil {
			return err
		}
		result = res
		return nil
	})
	return result, err
}

func (s *movementService) processConsumption(ctx context.Context, cmd CreateMovementCommand) (*domain.MovementResult, error) {
	var result *domain.MovementResult
	err := s.txManager.RunInTx(ctx, func(txCtx context.Context) error {
		if err := s.productRepo.LockProductBySKU(txCtx, cmd.SKU); err != nil {
			return err
		}
		activeBatches, err := s.stockRepo.GetActiveBatchesBySKUAndLocation(txCtx, cmd.SKU, cmd.LocationID)
		if err != nil {
			return err
		}
		allocations, _, err := calc.AllocateFEFO(activeBatches, cmd.Quantity, cmd.OperationDate, cmd.SKU, cmd.LocationID)
		if err != nil {
			return err
		}
		if err := s.recordProcessedDocument(txCtx, cmd); err != nil {
			return err
		}
		res, err := s.insertMovementsAndBuildResult(txCtx, cmd, s.buildConsumptionMovements(cmd, allocations))
		if err != nil {
			return err
		}
		result = res
		return nil
	})
	return result, err
}

func (s *movementService) processWriteoff(ctx context.Context, cmd CreateMovementCommand) (*domain.MovementResult, error) {
	if cmd.BatchID == "" {
		return nil, fmt.Errorf("batch_id is required for writeoff: %w", domain.ErrNotFound)
	}

	var result *domain.MovementResult
	err := s.txManager.RunInTx(ctx, func(txCtx context.Context) error {
		if err := s.productRepo.LockProductBySKU(txCtx, cmd.SKU); err != nil {
			return err
		}
		if err := s.verifyBatchAvailableStock(txCtx, cmd); err != nil {
			return err
		}
		if err := s.recordProcessedDocument(txCtx, cmd); err != nil {
			return err
		}
		res, err := s.insertMovementsAndBuildResult(txCtx, cmd, []domain.Movement{s.buildBaseMovement(cmd)})
		if err != nil {
			return err
		}
		result = res
		return nil
	})
	return result, err
}

func (s *movementService) processReturn(ctx context.Context, cmd CreateMovementCommand) (*domain.MovementResult, error) {
	if cmd.BatchID == "" {
		return nil, fmt.Errorf("batch_id is required for return: %w", domain.ErrNotFound)
	}

	var result *domain.MovementResult
	err := s.txManager.RunInTx(ctx, func(txCtx context.Context) error {
		if err := s.productRepo.LockProductBySKU(txCtx, cmd.SKU); err != nil {
			return err
		}
		if err := s.recordProcessedDocument(txCtx, cmd); err != nil {
			return err
		}
		res, err := s.insertMovementsAndBuildResult(txCtx, cmd, []domain.Movement{s.buildBaseMovement(cmd)})
		if err != nil {
			return err
		}
		result = res
		return nil
	})
	return result, err
}

func (s *movementService) processCorrection(ctx context.Context, cmd CreateMovementCommand) (*domain.MovementResult, error) {
	var result *domain.MovementResult
	err := s.txManager.RunInTx(ctx, func(txCtx context.Context) error {
		if err := s.productRepo.LockProductBySKU(txCtx, cmd.SKU); err != nil {
			return err
		}
		if err := s.recordProcessedDocument(txCtx, cmd); err != nil {
			return err
		}
		res, err := s.insertMovementsAndBuildResult(txCtx, cmd, []domain.Movement{s.buildBaseMovement(cmd)})
		if err != nil {
			return err
		}
		result = res
		return nil
	})
	return result, err
}

func (s *movementService) recordProcessedDocument(ctx context.Context, cmd CreateMovementCommand) error {
	return s.movementRepo.InsertProcessedDocument(ctx, domain.Movement{
		DocumentNo:    cmd.DocumentNo,
		OperationType: cmd.OperationType,
		OperationDate: cmd.OperationDate,
		SKU:           cmd.SKU,
		LocationID:    cmd.LocationID,
	})
}

func (s *movementService) upsertReceiptBatch(ctx context.Context, cmd CreateMovementCommand) error {
	batchID := cmd.BatchID
	if batchID == "" {
		batchID = fmt.Sprintf("BATCH-%s-%s", cmd.SKU, cmd.OperationDate.Format("20060102"))
	}

	return s.productRepo.UpsertBatch(ctx, domain.Batch{
		ID:            batchID,
		SKU:           cmd.SKU,
		ExpiryDate:    cmd.ExpiryDate,
		PurchasePrice: cmd.PurchasePrice,
		InvoiceNo:     cmd.InvoiceNo,
		ReceivedDate:  cmd.OperationDate,
	})
}

func (s *movementService) verifyBatchAvailableStock(ctx context.Context, cmd CreateMovementCommand) error {
	batches, err := s.stockRepo.GetActiveBatchesBySKUAndLocation(ctx, cmd.SKU, cmd.LocationID)
	if err != nil {
		return err
	}

	var batchStock decimal.Decimal
	found := false
	for _, b := range batches {
		if b.BatchID == cmd.BatchID {
			batchStock = b.Quantity
			found = true
			break
		}
	}

	if !found || batchStock.LessThan(cmd.Quantity) {
		return domain.NewInsufficientStockError(cmd.SKU, cmd.LocationID, cmd.Quantity, batchStock)
	}

	return nil
}

func (s *movementService) buildBaseMovement(cmd CreateMovementCommand) domain.Movement {
	batchID := cmd.BatchID
	if batchID == "" && cmd.OperationType == domain.OperationReceipt {
		batchID = fmt.Sprintf("BATCH-%s-%s", cmd.SKU, cmd.OperationDate.Format("20060102"))
	}

	return domain.Movement{
		DocumentNo:    cmd.DocumentNo,
		OperationDate: cmd.OperationDate,
		SKU:           cmd.SKU,
		LocationID:    cmd.LocationID,
		OperationType: cmd.OperationType,
		Quantity:      cmd.Quantity,
		BatchID:       batchID,
		Note:          cmd.Note,
	}
}

func (s *movementService) buildConsumptionMovements(cmd CreateMovementCommand, allocations []calc.FEFOAllocation) []domain.Movement {
	movements := make([]domain.Movement, 0, len(allocations))
	for _, alloc := range allocations {
		movements = append(movements, domain.Movement{
			DocumentNo:    cmd.DocumentNo,
			OperationDate: cmd.OperationDate,
			SKU:           cmd.SKU,
			LocationID:    cmd.LocationID,
			OperationType: domain.OperationConsume,
			Quantity:      alloc.AllocatedQty,
			BatchID:       alloc.BatchID,
			Note:          cmd.Note,
		})
	}
	return movements
}

func (s *movementService) buildMovementResult(cmd CreateMovementCommand, ids []int64, currentStock decimal.Decimal) *domain.MovementResult {
	var primaryID int64
	if len(ids) > 0 {
		primaryID = ids[0]
	}

	return &domain.MovementResult{
		ID:            primaryID,
		MovementIDs:   ids,
		DocumentNo:    cmd.DocumentNo,
		SKU:           cmd.SKU,
		LocationID:    cmd.LocationID,
		OperationType: cmd.OperationType,
		Quantity:      cmd.Quantity,
		CurrentStock:  currentStock,
		CreatedAt:     time.Now(),
	}
}

func (s *movementService) insertMovementsAndBuildResult(
	ctx context.Context,
	cmd CreateMovementCommand,
	movements []domain.Movement,
) (*domain.MovementResult, error) {
	ids, err := s.movementRepo.InsertMovements(ctx, movements)
	if err != nil {
		return nil, err
	}

	currentStock, err := s.stockRepo.GetCurrentStock(ctx, cmd.SKU, cmd.LocationID)
	if err != nil {
		return nil, err
	}

	return s.buildMovementResult(cmd, ids, currentStock), nil
}
