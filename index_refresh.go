package main

import (
	"net/http"

	"github.com/jehiah/legislation.support/internal/concurrentlimit"
	"github.com/jehiah/legislation.support/internal/resolvers"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

// InternalRefresh handles a periodic request to refresh data
// at /internal/refresh
func (a *App) InternalRefresh(w http.ResponseWriter, r *http.Request) {
	if !a.devMode {
		if r.Header.Get("X-Cloudscheduler") != "true" {
			log.Printf("InternalRefresh headers %#v", r.Header)
			http.Error(w, "Not Found", 404)
			return
		}
	}

	r.ParseForm()
	ctx := r.Context()
	limit := 500
	bills, err := a.GetStaleBills(ctx, limit)
	if err != nil {
		log.Printf("err %s", err)
		http.Error(w, err.Error(), 500)
		return
	}

	limiter := concurrentlimit.NewConcurrentLimit(5)

	// if any bills are stale, refresh (some) of them - error should not be fatal
	var wg errgroup.Group
	for i := range bills {
		i := i
		l := bills[i]

		wg.Go(func() error {
			return limiter.Run(func() error {
				udpatedLeg, err := resolvers.Resolvers.Find(l.Body).Refresh(ctx, l.ID)
				if err != nil {
					return err
				}
				if r.Form.Get("dry_run") == "true" {
					log.Printf("dry_run %#v", *udpatedLeg)
				} else {
					err = a.UpdateBill(ctx, l, *udpatedLeg)
					if err != nil {
						return err
					}
				}
				return nil
			})
		})
	}
	if err := wg.Wait(); err != nil {
		log.Printf("err %s", err)
		http.Error(w, err.Error(), 500)
		return
	}

	w.Write([]byte("done"))
}
