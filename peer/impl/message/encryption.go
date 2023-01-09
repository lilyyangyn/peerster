package message

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"

	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
	"golang.org/x/xerrors"
)

type EncryptionModule struct {
	*MessageModule
	conf *peer.Configuration

	pubkeyStore *PubkeyController
	privkey     *types.Privkey
}

func NewEncryptionModule(conf *peer.Configuration, messageModue *MessageModule) *EncryptionModule {
	m := EncryptionModule{
		MessageModule: messageModue,
		conf:          conf,
	}

	privkey, pubkey, err := m.generateRSAKeyPair(2048)
	if err != nil {
		panic(err)
	}
	m.privkey = privkey
	m.pubkeyStore = NewPubkeyController(
		conf.Socket.GetAddress(),
		pubkey,
	)

	// message registery
	m.conf.MessageRegistry.RegisterMessageCallback(types.PubkeyMessage{}, m.ProcessPubkeyMsg)
	m.conf.MessageRegistry.RegisterMessageCallback(types.EncryptedMessage{}, m.ProcessEntryptedMsg)

	return &m
}

/** Feature Functions **/

// GetPubkeyString returns a string to descript the pubkey
func (m *EncryptionModule) GetPubkeyString() (string, error) {
	pubBytes, err := x509.MarshalPKIXPublicKey((*rsa.PublicKey)(&m.privkey.PublicKey))
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(pubBytes), nil
}

// SendEncryptedMessage broadcast an encrypted message in private msg
func (m *EncryptionModule) SendEncryptedMessage(msg transport.Message, to string) error {
	// encrypt message
	encryptedMsg, err := m.encryptMsg(msg, to)
	if err != nil {
		return err
	}

	encryptedMsgMarshal, err := m.CreateMsg(encryptedMsg)
	if err != nil {
		return err
	}

	// wrap in private msg
	privMsg := types.PrivateMessage{
		Recipients: map[string]struct{}{to: {}},
		Msg:        &encryptedMsgMarshal,
	}
	privMsgMarshal, err := m.CreateMsg(privMsg)
	if err != nil {
		return err
	}

	// send in rumor
	err = m.Broadcast(privMsgMarshal)

	return err
}

// EncryptAsymetric encrypts value using peer's pubkey
func (m *EncryptionModule) EncryptAsymetric(value []byte, pubkey types.Pubkey) ([]byte, error) {
	pub := rsa.PublicKey(pubkey)

	hash := sha256.New()
	msgLen := len(value)
	step := pub.Size() - 2*hash.Size() - 2
	var encryptedBytes []byte

	for start := 0; start < msgLen; start += step {
		finish := start + step
		if finish > msgLen {
			finish = msgLen
		}

		encryptedBlockBytes, err := rsa.EncryptOAEP(hash, rand.Reader, &pub, value[start:finish], nil)
		if err != nil {
			return nil, err
		}

		encryptedBytes = append(encryptedBytes, encryptedBlockBytes...)
	}

	return encryptedBytes, nil
}

// DecryptAsymetric decrypts value using self's pubkey
func (m *EncryptionModule) DecryptAsymetric(value []byte) ([]byte, error) {
	priv := (*rsa.PrivateKey)(m.privkey)

	hash := sha256.New()
	msgLen := len(value)
	step := priv.PublicKey.Size()
	var decryptedBytes []byte

	for start := 0; start < msgLen; start += step {
		finish := start + step
		if finish > msgLen {
			finish = msgLen
		}

		decryptedBlockBytes, err := rsa.DecryptOAEP(hash, rand.Reader, priv, value[start:finish], nil)
		if err != nil {
			return nil, err
		}

		decryptedBytes = append(decryptedBytes, decryptedBlockBytes...)
	}

	return decryptedBytes, nil
}

// SetPubkeyEntry sets the publickey entry
func (m *EncryptionModule) SetPubkeyEntry(origin string, pubkey *types.Pubkey) {
	// Delete the record if no relayAddr
	if pubkey == nil {
		m.pubkeyStore.remove(origin)
		return
	}
	// Otherwise, update the table
	m.pubkeyStore.add(origin, pubkey)
}

// GetSelfPubkey returns the public key of myself
func (m *EncryptionModule) GetPubkey() types.Pubkey {
	return types.Pubkey(m.privkey.PublicKey)
}

// GetPubkeyStore returns the node's pubkey store. It should be a copy.
func (m *EncryptionModule) GetPubkeyStore() peer.PubkeyStore {
	return m.pubkeyStore.getAll()
}

/** Private Helpfer Functions **/

// generateKeyPair generates privkey-pubkey pair
func (m *EncryptionModule) generateRSAKeyPair(bits int) (privKey *types.Privkey, pubKey *types.Pubkey, err error) {
	rsapriv, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return privKey, pubKey, err
	}

	return (*types.Privkey)(rsapriv), (*types.Pubkey)(&rsapriv.PublicKey), nil
}

// encryptMsg encrypts message using peer's pubkey
func (m *EncryptionModule) encryptMsg(msg transport.Message, peer string) (*types.EncryptedMessage, error) {
	ptxt, err := json.Marshal(&msg)
	if err != nil {
		return nil, err
	}
	pubkey, ok := m.pubkeyStore.get(peer)
	if !ok {
		return nil, xerrors.Errorf("no public key for peer %s", peer)
	}
	encMsg, err := m.EncryptAsymetric(ptxt, pubkey)
	if err != nil {
		return nil, err
	}

	return (*types.EncryptedMessage)(&encMsg), nil
}

// decryptMsg decrypts message using privkey
func (m *EncryptionModule) decryptMsg(encMsg types.EncryptedMessage) (msg *transport.Message, err error) {
	ptxt, err := m.DecryptAsymetric(encMsg)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(ptxt, &msg)
	if err != nil {
		return nil, err
	}

	return msg, nil
}

// createPubkeyMsg creates a marshaled pubkey message
func (m *EncryptionModule) createPubkeyMsg() (types.PubkeyMessage, bool) {
	if m.privkey == nil {
		return types.PubkeyMessage{}, false
	}
	msg := types.PubkeyMessage{
		Origin: m.conf.Socket.GetAddress(),
		Pubkey: types.Pubkey(m.privkey.PublicKey),
	}

	return msg, true
}
