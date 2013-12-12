package metadataservice

type Metadata struct {
	Index           string
	Guid            string
	SyslogDrainUrls []string
}

type MetaDataService interface {
	Lookup(wardenHandle string) (Metadata, error)
}
