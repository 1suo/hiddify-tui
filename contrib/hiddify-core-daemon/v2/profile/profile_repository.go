package profile

import (
	context "context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hiddify/hiddify-core/v2/config"
	"github.com/hiddify/hiddify-core/v2/db"
	"github.com/hiddify/hiddify-core/v2/hcommon"
	"github.com/hiddify/hiddify-core/v2/hcommon/request"
	hcore "github.com/hiddify/hiddify-core/v2/hcore"
	"github.com/sagernet/sing-box/option"
)

const (
	profilesDirName = "data/profiles"
)

type ProfileRepositoryServer struct {
	UnimplementedProfileServiceServer
}

func ClearActiveProfile(id string) error {
	table := db.GetTable[hcommon.AppSettings]()
	active, err := table.Get("active_profile")
	if err != nil || active == nil {
		return nil
	}
	value, ok := active.Value.(string)
	if !ok || value != id {
		return nil
	}
	return table.Delete("active_profile")
}

func (s *ProfileRepositoryServer) GetProfile(ctx context.Context, req *ProfileRequest) (*ProfileResponse, error) {
	var profile *ProfileEntity
	var err error

	switch {
	case req.Id != "":
		profile, err = GetById(req.Id)
	case req.Name != "":
		profile, err = GetByName(req.Name)
	case req.Url != "":
		profile, err = GetByUrl(ctx, req.Url)
	default:
		return nil, fmt.Errorf("invalid request: %v", req)
	}

	if err != nil {
		return nil, fmt.Errorf("error fetching profile: %v", err)
	}

	return &ProfileResponse{Profile: profile}, nil
}

func (s *ProfileRepositoryServer) AddProfile(ctx context.Context, req *AddProfileRequest) (*ProfileResponse, error) {
	var profile *ProfileEntity
	var err error

	switch {
	case req.Url != "":
		profile, err = AddByUrl(ctx, req.Url, req.Name, req.MarkAsActive)

	case req.Content != "":
		profile, err = AddByContent(ctx, req.Content, req.Name, req.MarkAsActive)
	default:
		return nil, fmt.Errorf("invalid request: %v", req)
	}

	if err != nil {
		return nil, fmt.Errorf("error fetching profile: %v", err)
	}

	return &ProfileResponse{Profile: profile}, nil
}

func (s *ProfileRepositoryServer) DeleteProfile(ctx context.Context, req *ProfileRequest) (*hcommon.Response, error) {
	var err error
	switch {
	case req.Id != "":
		err = DeleteById(req.Id)
	default:
		profile, err1 := s.GetProfile(ctx, req)

		if profile.Profile == nil {
			err = fmt.Errorf("error deleting profile: %v", err1)
		} else {
			err = DeleteById(profile.Profile.Id)
		}
	}

	if err != nil {
		return &hcommon.Response{Message: err.Error(), Code: hcommon.ResponseCode_FAILED}, fmt.Errorf("error deleting profile: %v", err)
	}

	return &hcommon.Response{Code: hcommon.ResponseCode_OK}, nil
}

func (s *ProfileRepositoryServer) SetActiveProfile(ctx context.Context, req *ProfileRequest) (*hcommon.Response, error) {
	var err error
	switch {
	case req.Id != "":

		var profile *ProfileEntity
		profile, err = GetById(req.Id)
		if err == nil {
			err = SetActiveProfile(profile)
		}
	default:

		var profile *ProfileResponse
		profile, err = s.GetProfile(ctx, req)

		if profile.Profile == nil {
			err = fmt.Errorf("error setting profile as active: %v", err)
		} else {
			err = SetActiveProfile(profile.Profile)
		}
	}

	if err != nil {
		return &hcommon.Response{Message: err.Error(), Code: hcommon.ResponseCode_FAILED}, fmt.Errorf("error setting profile as active: %v", err)
	}

	return &hcommon.Response{Code: hcommon.ResponseCode_OK}, nil
}

func (s *ProfileRepositoryServer) GetProfiles(ctx context.Context, req *hcommon.Empty) (*MultiProfilesResponse, error) {
	profiles, err := GetAll()
	if err != nil {
		return &MultiProfilesResponse{ResponseCode: hcommon.ResponseCode_FAILED, Message: err.Error()}, fmt.Errorf("error fetching profiles: %v", err)
	}
	return &MultiProfilesResponse{Profiles: profiles}, nil
}

func (s *ProfileRepositoryServer) UpdateProfile(ctx context.Context, req *ProfileEntity) (*hcommon.Response, error) {
	err := UpdateProfile(req)
	if err != nil {
		return &hcommon.Response{Message: err.Error(), Code: hcommon.ResponseCode_FAILED}, fmt.Errorf("error updating profile: %v", err)
	}
	return &hcommon.Response{Code: hcommon.ResponseCode_OK}, nil
}

func GetAll() ([]*ProfileEntity, error) {
	table := db.GetTable[ProfileEntity]()
	allEntities, err := table.All()
	return allEntities, err
}

func (s *ProfileRepositoryServer) GetActiveProfile(ctx context.Context, req *hcommon.Empty) (*ProfileResponse, error) {
	profile, err := GetActiveProfile()
	if err != nil {
		return &ProfileResponse{ResponseCode: hcommon.ResponseCode_FAILED, Message: err.Error()}, fmt.Errorf("error fetching active profile: %v", err)
	}
	return &ProfileResponse{Profile: profile}, nil
}

func GetActiveProfile() (*ProfileEntity, error) {
	table := db.GetTable[hcommon.AppSettings]()
	active, err := table.Get("active_profile")
	if err != nil {
		return nil, err
	}
	prof, err := GetById(active.Value.(string))
	if err != nil {
		return nil, err
	}
	return prof, nil
}

func SetActiveProfile(entity *ProfileEntity) error {
	table := db.GetTable[hcommon.AppSettings]()
	return table.UpdateInsert(&hcommon.AppSettings{
		Id:    "active_profile",
		Value: entity.Id,
	})
}

func GetById(id string) (*ProfileEntity, error) {
	table := db.GetTable[ProfileEntity]()
	entity, err := table.Get(id)
	if err != nil {
		return nil, fmt.Errorf("error fetching profile by ID: %v", err)
	}
	return entity, nil
}

func GetByName(name string) (*ProfileEntity, error) {
	table := db.GetTable[ProfileEntity]()
	allEntities, err := table.All()
	for _, entity := range allEntities {
		if entity.Name == name {
			return entity, nil
		}
	}

	return nil, fmt.Errorf("error fetching profile by ID: %v", err)
}

func GetByUrl(ctx context.Context, url string) (*ProfileEntity, error) {
	table := db.GetTable[ProfileEntity]()
	allEntities, err := table.All()
	for _, entity := range allEntities {
		if entity.Url == url {
			return entity, nil
		}
	}

	return nil, fmt.Errorf("error fetching profile by ID: %v", err)
}

func AddByUrl(ctx context.Context, url string, optionalName string, markAsActive bool) (*ProfileEntity, error) {
	existingProfile, _ := GetByUrl(ctx, url)
	if existingProfile != nil {
		// If the profile already exists, update it
		return existingProfile, UpdateSubscription(existingProfile, false)
	}

	profileId := generateUuid()

	// Attempt to download the profile content
	content, err := downloadProfileContent(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("error downloading profile: %v", err)
	}

	err = UpdateContent(ctx, profileId, content.Body)
	if err != nil {
		return nil, fmt.Errorf("error updating profile content: %v", err)
	}

	newProfile := &ProfileEntity{
		Id: profileId,

		LastUpdate: time.Now().UnixMilli(),
		Url:        url,
	}
	newProfile.Parse(content.Header)
	if optionalName != "" {
		newProfile.Name = optionalName
	}
	table := db.GetTable[ProfileEntity]()
	if err := table.UpdateInsert(newProfile); err != nil {
		return nil, fmt.Errorf("error inserting new profile into the database: %v", err)
	}

	if markAsActive {
		SetActiveProfile(newProfile)
	}
	return newProfile, nil
}

// downloadProfileContent handles the download logic
func downloadProfileContent(ctx context.Context, url string) (*request.Response, error) {
	resp, err := request.Send(request.Request{
		Url:       url,
		Method:    request.GET,
		SocksPort: 12334,
		Timeout:   5 * time.Second,
	})
	if resp == nil {
		resp, err = request.Send(request.Request{
			Url:     url,
			Method:  request.GET,
			Timeout: 5 * time.Second,
		})
		if resp == nil {
			instance, err1 := hcore.RunInstance(ctx, config.DefaultHiddifyOptions(), &option.Options{})
			if err1 != nil {
				return nil, fmt.Errorf("%v,error running instance: %v", err, err1)
			}
			instance.PingCloudflare()
			resp, err1 = request.Send(request.Request{
				Url:       url,
				Method:    request.GET,
				Timeout:   5 * time.Second,
				SocksPort: instance.ListenPort,
			})
			if err1 != nil {
				err = fmt.Errorf("%v, Fragment: %v", err, err1)
			}
		}
	}
	if resp == nil {
		return nil, err
	}
	if resp.StatusCode == 401 {
		return nil, fmt.Errorf("Authentication error: %v", resp.StatusCode)
	}
	contentHeaders := parseHeadersFromContent(resp.Body)
	for k, v := range contentHeaders {
		resp.Header.Set(k, v[0])
	}
	return resp, nil
}

func UpdateContent(ctx context.Context, profileId, content string) error {
	if err := os.MkdirAll(profilesDirName, 0o700); err != nil {
		return fmt.Errorf("create profiles directory: %w", err)
	}

	_, err := hcore.Parse(ctx, &hcore.ParseRequest{
		Content: content,
	})
	if err != nil {
		return err
	}

	path, err := ContentPath(profileId)
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(profilesDirName, ".profile-*.tmp")
	if err != nil {
		return fmt.Errorf("create profile content: %w", err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return fmt.Errorf("protect profile content: %w", err)
	}
	if _, err := temporary.WriteString(content); err != nil {
		temporary.Close()
		return fmt.Errorf("write profile content: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("sync profile content: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close profile content: %w", err)
	}
	if err := os.Rename(temporaryName, path); err != nil {
		return fmt.Errorf("replace profile content: %w", err)
	}
	return nil
}

func AddByContent(ctx context.Context, content, name string, markAsActive bool) (*ProfileEntity, error) {
	profileId := generateUuid()

	err := UpdateContent(ctx, profileId, content)
	if err != nil {
		return nil, err
	}

	newProfile := &ProfileEntity{
		Id:         profileId,
		Name:       name,
		LastUpdate: time.Now().UnixMilli(),
	}

	table := db.GetTable[ProfileEntity]()
	if err := table.UpdateInsert(newProfile); err != nil {
		return nil, fmt.Errorf("error inserting new profile into the database: %v", err)
	}
	if markAsActive {
		SetActiveProfile(newProfile)
	}
	return newProfile, nil
}

func UpdateSubscription(baseProfile *ProfileEntity, patchBaseProfile bool) error {
	if baseProfile == nil || baseProfile.Url == "" {
		return fmt.Errorf("remote profile URL is required")
	}
	content, err := downloadProfileContent(context.Background(), baseProfile.Url)
	if err != nil {
		return fmt.Errorf("download profile: %w", err)
	}
	if err := UpdateContent(context.Background(), baseProfile.Id, content.Body); err != nil {
		return fmt.Errorf("validate profile: %w", err)
	}
	if patchBaseProfile {
		baseProfile.Parse(content.Header)
	}
	baseProfile.LastUpdate = time.Now().UnixMilli()
	if err := UpdateProfile(baseProfile); err != nil {
		return fmt.Errorf("update profile metadata: %w", err)
	}
	return nil
}

func Patch(profile *ProfileEntity) error {
	// Implement patch logic
	return nil
}

func DeleteById(id string) error {
	table := db.GetTable[ProfileEntity]()
	path, err := ContentPath(id)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove profile content: %w", err)
	}
	if err := ClearActiveProfile(id); err != nil {
		return fmt.Errorf("clear active profile: %w", err)
	}
	return table.Delete(id)
}

// ContentPath returns the daemon-owned location for a validated profile ID.
// Callers must obtain the ID from the profile store rather than from a client
// supplied path.
func ContentPath(id string) (string, error) {
	if id == "" || filepath.Base(id) != id {
		return "", fmt.Errorf("invalid profile id")
	}
	return filepath.Join(profilesDirName, id+".info"), nil
}

func UpdateProfile(profile *ProfileEntity) error {
	table := db.GetTable[ProfileEntity]()
	return table.UpdateInsert(profile)
}
