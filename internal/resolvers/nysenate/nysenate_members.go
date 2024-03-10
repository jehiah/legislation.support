package nysenate

import (
	"context"

	"github.com/jehiah/legislation.support/internal/legislature"
)

func (a *NYAssembly) Members(ctx context.Context, session legislature.Session) ([]legislature.Member, error) {
	return a.api.GetMembers(ctx, session, assemblyChamber)
}

func (a *NYSenate) Members(ctx context.Context, session legislature.Session) ([]legislature.Member, error) {
	return a.api.GetMembers(ctx, session, senateChamber)
}
