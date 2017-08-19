package gh

import (
	"errors"
	"fmt"

	"github.com/google/go-github/github"
)

const (
	// Pending Check state
	Pending = "pending"
	// Success Check state
	Success = "success"

	// Approved Review state
	Approved = "APPROVED"
	// Commented Review state
	Commented = "COMMENTED"
)

// HasReviewsApprove check if a PR have the required number of review
func (g *GHub) HasReviewsApprove(pr *github.PullRequest, minReview int) error {
	if minReview != 0 {

		owner := pr.Base.Repo.Owner.GetLogin()
		repositoryName := pr.Base.Repo.GetName()
		prNumber := pr.GetNumber()

		opt := &github.ListOptions{
			PerPage: 100,
		}

		reviewsState := make(map[string]string)
		for {
			reviews, resp, err := g.client.PullRequests.ListReviews(g.ctx, owner, repositoryName, prNumber, opt)
			if err != nil {
				return err
			}

			for _, review := range reviews {
				if review.GetState() != Commented {
					reviewsState[review.User.GetLogin()] = review.GetState()
				}
			}
			if resp.NextPage == 0 {
				break
			}
			opt.Page = resp.NextPage
		}

		if len(reviewsState) < minReview {
			return fmt.Errorf("Need more review [%v/2]", len(reviewsState))
		}

		for login, state := range reviewsState {
			if state != Approved {
				return fmt.Errorf("%s by %s", state, login)
			}
		}
	}
	return nil
}

// IsUpToDateBranch check if a PR is up to date
func (g *GHub) IsUpToDateBranch(pr *github.PullRequest) (bool, error) {

	branch := pr.Base.GetRef()

	ref, _, err := g.client.Git.GetRef(
		g.ctx,
		pr.Base.Repo.Owner.GetLogin(),
		pr.Base.Repo.GetName(),
		fmt.Sprintf("heads/%s", branch))
	if err != nil {
		return false, err
	}

	prSHA := pr.Base.GetSHA()
	branchHeadSHA := ref.Object.GetSHA()

	return prSHA == branchHeadSHA, nil
}

// GetStatus provide checks status (CI)
func (g *GHub) GetStatus(pr *github.PullRequest) (string, error) {

	owner := pr.Base.Repo.Owner.GetLogin()
	repositoryName := pr.Base.Repo.GetName()
	prRef := pr.Head.GetSHA()

	sts, _, err := g.client.Repositories.GetCombinedStatus(g.ctx, owner, repositoryName, prRef, nil)
	if err != nil {
		return "", err
	}

	if sts.GetState() == Success {
		return sts.GetState(), nil
	}

	// pending: if there are no statuses or a context is pending
	// https://developer.github.com/v3/repos/statuses/#get-the-combined-status-for-a-specific-ref
	if sts.GetState() == Pending {
		if sts.GetTotalCount() == 0 {
			return Success, nil
		}
		return sts.GetState(), nil
	}

	statuses, _, err := g.client.Repositories.ListStatuses(g.ctx, owner, repositoryName, prRef, nil)
	if err != nil {
		return "", err
	}
	var summary string
	for _, stat := range statuses {
		if stat.GetState() != Success {
			summary += stat.GetDescription() + "\n"
		}
	}
	return "", errors.New(summary)
}
