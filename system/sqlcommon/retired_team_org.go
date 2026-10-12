package sqlcommon

import (
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
)

// RetiredCreateOrganization keeps the ApitoSystemDB method compiling after the
// named Organization table is dropped. It does not persist.
func RetiredCreateOrganization(org *models.Organization) (*models.Organization, error) {
	if org == nil {
		org = &models.Organization{}
	}
	if org.ID == "" {
		org.ID = utility.NewID()
	}
	return org, nil
}

// RetiredCreateTeam keeps CreateTeam compiling after the named Team table is dropped.
func RetiredCreateTeam(team *models.Team) (*models.Team, error) {
	if team == nil {
		team = &models.Team{}
	}
	if team.ID == "" {
		team.ID = utility.NewID()
	}
	return team, nil
}

// RetiredFindUserOrganizations returns no organizations.
func RetiredFindUserOrganizations() ([]*models.Organization, error) {
	return []*models.Organization{}, nil
}

// RetiredGetOrganizations returns an empty search payload.
func RetiredGetOrganizations() (*models.SearchResponse[models.Organization], error) {
	return &models.SearchResponse[models.Organization]{Results: []*models.Organization{}}, nil
}

// RetiredFindOrganizationAdmin returns no admin after the org graph is retired.
func RetiredFindOrganizationAdmin() (*models.SystemUser, error) {
	return nil, nil
}

// RetiredGetProjectTeams returns no named team.
func RetiredGetProjectTeams() (*models.Team, error) {
	return nil, nil
}

// RetiredAddTeamMetaInfo is a no-op; membership metadata lives on user_projects.
func RetiredAddTeamMetaInfo(docs []*models.SystemUser) ([]*models.SystemUser, error) {
	return docs, nil
}

// RetiredNoop is Assign/Remove on the retired org graph.
func RetiredNoop() error {
	return nil
}
