package jellyfin

import "time"

// SystemInfo represents Jellyfin server system information.
type SystemInfo struct {
	ServerName             string `json:"ServerName"`
	Version                string `json:"Version"`
	ID                     string `json:"Id"`
	OperatingSystem        string `json:"OperatingSystem"`
	HasPendingRestart      bool   `json:"HasPendingRestart"`
	HasUpdateAvailable     bool   `json:"HasUpdateAvailable"`
	SupportsLibraryMonitor bool   `json:"SupportsLibraryMonitor"`
	CanSelfRestart         bool   `json:"CanSelfRestart"`
	CanLaunchWebBrowser    bool   `json:"CanLaunchWebBrowser"`
	LocalAddress           string `json:"LocalAddress"`
}

// Library represents a Jellyfin media library (virtual folder).
type Library struct {
	Name            string   `json:"Name"`
	CollectionType  string   `json:"CollectionType"` // movies, tvshows, music, etc.
	ItemID          string   `json:"ItemId"`
	Locations       []string `json:"Locations"`
	RefreshStatus   string   `json:"RefreshStatus"`
	RefreshProgress float64  `json:"RefreshProgress"`
}

// VirtualFoldersResponse wraps the library list response.
type VirtualFoldersResponse []Library

// MediaItem represents a Jellyfin media item (movie, episode, song, etc.).
type MediaItem struct {
	Name              string        `json:"Name"`
	ID                string        `json:"Id"`
	Type              string        `json:"Type"` // Movie, Episode, Series, Album, Audio, etc.
	Overview          string        `json:"Overview"`
	ParentID          string        `json:"ParentId"`
	SeriesName        string        `json:"SeriesName"`
	SeasonName        string        `json:"SeasonName"`
	IndexNumber       int           `json:"IndexNumber"`
	ParentIndexNumber int           `json:"ParentIndexNumber"`
	ProductionYear    int           `json:"ProductionYear"`
	CommunityRating   float64       `json:"CommunityRating"`
	OfficialRating    string        `json:"OfficialRating"`
	RunTimeTicks      int64         `json:"RunTimeTicks"`
	Genres            []string      `json:"Genres"`
	Studios           []NameIDPair  `json:"Studios"`
	People            []PersonInfo  `json:"People"`
	MediaSources      []MediaSource `json:"MediaSources"`
	UserData          *UserData     `json:"UserData"`
	PremiereDate      *time.Time    `json:"PremiereDate"`
	DateCreated       *time.Time    `json:"DateCreated"`
	ChildCount        int           `json:"ChildCount"`
	Path              string        `json:"Path"`
}

// NameIDPair is a simple name+ID struct used for studios, genres, etc.
type NameIDPair struct {
	Name string `json:"Name"`
	ID   string `json:"Id"`
}

// PersonInfo represents a person (actor, director, etc.) associated with media.
type PersonInfo struct {
	Name string `json:"Name"`
	ID   string `json:"Id"`
	Role string `json:"Role"`
	Type string `json:"Type"` // Actor, Director, Writer, etc.
}

// MediaSource represents a media file source.
type MediaSource struct {
	ID        string        `json:"Id"`
	Path      string        `json:"Path"`
	Container string        `json:"Container"`
	Size      int64         `json:"Size"`
	Bitrate   int           `json:"Bitrate"`
	Streams   []MediaStream `json:"MediaStreams"`
}

// MediaStream represents an audio, video, or subtitle stream.
type MediaStream struct {
	Type         string `json:"Type"` // Video, Audio, Subtitle
	Codec        string `json:"Codec"`
	Language     string `json:"Language"`
	DisplayTitle string `json:"DisplayTitle"`
	BitRate      int    `json:"BitRate"`
	Width        int    `json:"Width"`
	Height       int    `json:"Height"`
	Channels     int    `json:"Channels"`
}

// UserData represents user-specific data for an item.
type UserData struct {
	PlaybackPositionTicks int64  `json:"PlaybackPositionTicks"`
	PlayCount             int    `json:"PlayCount"`
	IsFavorite            bool   `json:"IsFavorite"`
	Played                bool   `json:"Played"`
	LastPlayedDate        string `json:"LastPlayedDate"`
}

// ItemsResponse is the standard paginated response from Jellyfin.
type ItemsResponse struct {
	Items            []MediaItem `json:"Items"`
	TotalRecordCount int         `json:"TotalRecordCount"`
	StartIndex       int         `json:"StartIndex"`
}

// Session represents an active Jellyfin playback session.
type Session struct {
	ID                    string     `json:"Id"`
	UserName              string     `json:"UserName"`
	Client                string     `json:"Client"`
	DeviceName            string     `json:"DeviceName"`
	DeviceID              string     `json:"DeviceId"`
	ApplicationVersion    string     `json:"ApplicationVersion"`
	NowPlayingItem        *MediaItem `json:"NowPlayingItem"`
	PlayState             *PlayState `json:"PlayState"`
	LastActivityDate      string     `json:"LastActivityDate"`
	SupportsRemoteControl bool       `json:"SupportsRemoteControl"`
}

// PlayState represents the playback state of a session.
type PlayState struct {
	PositionTicks int64  `json:"PositionTicks"`
	CanSeek       bool   `json:"CanSeek"`
	IsPaused      bool   `json:"IsPaused"`
	IsMuted       bool   `json:"IsMuted"`
	VolumeLevel   int    `json:"VolumeLevel"`
	PlayMethod    string `json:"PlayMethod"`
	MediaSourceID string `json:"MediaSourceId"`
}

// ActivityLogEntry represents an entry in the Jellyfin activity log.
type ActivityLogEntry struct {
	ID       int64  `json:"Id"`
	Name     string `json:"Name"`
	Type     string `json:"Type"`
	Date     string `json:"Date"`
	UserID   string `json:"UserId"`
	Severity string `json:"Severity"` // Information, Warning, Error
	Overview string `json:"ShortOverview"`
}

// ActivityLogResponse wraps the activity log list response.
type ActivityLogResponse struct {
	Items            []ActivityLogEntry `json:"Items"`
	TotalRecordCount int                `json:"TotalRecordCount"`
}
