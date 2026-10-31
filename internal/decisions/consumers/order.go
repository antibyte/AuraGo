package consumers

// Order lists every consumer in presentation order (router first).
var Order = []ID{Router, MemoryRerank, Appraisal, SalienceGate, MailTriage, WebhookQuestion, PersonWake, IndexerImages, GameMakerVisual}
