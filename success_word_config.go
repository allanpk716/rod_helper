package rod_helper

type SuccessWordsConfig struct {
	WordsConfig
}

type FailWordsConfig struct {
	WordsConfig
}

type WordsConfig struct {
	Enable     bool
	Words      []string
	WordsRegex []string
}
