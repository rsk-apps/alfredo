package sqlite

import (
	"context"
	"errors"
	"testing"

	"github.com/rafaelsoares/alfredo/internal/petcare/service"
)

func TestTxRunnerCommitsAndRollsBack(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	runner := NewTxRunner(db)

	err := runner.WithinTx(ctx, func(pets *service.PetService, _ *service.VaccineService, _ *service.TreatmentService, _ *service.DoseService) error {
		_, err := pets.Create(ctx, service.CreatePetInput{Name: "Luna", Species: "dog"})
		return err
	})
	if err != nil {
		t.Fatalf("WithinTx commit path: %v", err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pets`).Scan(&count); err != nil {
		t.Fatalf("count pets: %v", err)
	}
	if count != 1 {
		t.Fatalf("pet count = %d, want committed pet", count)
	}

	errRollback := errors.New("rollback")
	err = runner.WithinTx(ctx, func(pets *service.PetService, _ *service.VaccineService, _ *service.TreatmentService, _ *service.DoseService) error {
		if _, err := pets.Create(ctx, service.CreatePetInput{Name: "Milo", Species: "cat"}); err != nil {
			return err
		}
		return errRollback
	})
	if !errors.Is(err, errRollback) {
		t.Fatalf("WithinTx rollback error = %v, want sentinel", err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM pets`).Scan(&count); err != nil {
		t.Fatalf("count pets after rollback: %v", err)
	}
	if count != 1 {
		t.Fatalf("pet count = %d, want rollback to keep only committed pet", count)
	}
}
