package main

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

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
	ctx, cancel := context.WithTimeout(r.Context(), time.Second*45)
	defer cancel()
	limit := 500
	if r.Form.Get("limit") != "" {
		limit, _ = strconv.Atoi(r.Form.Get("limit"))
		if limit > 2000 || limit < 1 {
			limit = 500
		}
	}

	bills, err := a.GetStaleBills(ctx, limit)
	if err != nil {
		log.Printf("err %s", err)
		http.Error(w, err.Error(), 500)
		return
	}

	limiter := concurrentlimit.NewConcurrentLimit(5)

	var skipped int64
	// if any bills are stale, refresh (some) of them - error should not be fatal
	var wg errgroup.Group
	for i := range bills {
		i := i
		l := bills[i]

		wg.Go(func() error {
			return limiter.Run(func() error {
				select {
				case <-ctx.Done():
					atomic.AddInt64(&skipped, 1)
					return nil
				default:
				}
				udpatedLeg, err := resolvers.Resolvers.Find(l.Body).Refresh(ctx, l.ID)
				if err != nil {
					return err
				}
				if r.Form.Get("dry_run") == "true" {
					log.Printf("dry_run %#v", *udpatedLeg)
				} else {
					staleSameAs, err := a.UpdateBill(ctx, l, *udpatedLeg)
					if err != nil {
						return err
					}
					if staleSameAs {
						// refresh the sameAs bill (if needed)
						sameAsBody := resolvers.Bodies[l.Body].Bicameral
						sameBill, err := resolvers.Resolvers.Find(sameAsBody).Refresh(ctx, udpatedLeg.SameAs)
						if err != nil {
							return err
						}
						_, err = a.SaveBill(ctx, *sameBill)
						if err != nil {
							return err
						}
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
	output := "done"
	if skipped > 0 {
		log.Printf("refresh skipped %d due to timeout", skipped)
		output = fmt.Sprintf("done (skipped %d due to timeout)", skipped)
	}

	w.Write([]byte(output))
}
