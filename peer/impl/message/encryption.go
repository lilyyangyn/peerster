package message

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha512"
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

	return &m
}

/** Feature Functions **/

// BroadcastEncryptedMessage broadcast an encrypted message in private msg
func (m *EncryptionModule) BroadcastEncryptedMessage(msg transport.Message, to string) error {
	// encrypt message
	encryptedMsg, err := m.encryptWithPubkey(msg, to)
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

/** Message Handler **/

// ProcessPubkeyMsg is a callback function to handle the received pubkey message
func (m *EncryptionModule) ProcessPubkeyMsg(msg types.Message, pkt transport.Packet) error {
	pubkeyMsg, ok := msg.(*types.PubkeyMessage)
	if !ok {
		return xerrors.Errorf("wrong type: %T", msg)
	}

	// store pubkey into pubkeyStore
	m.pubkeyStore.add(pubkeyMsg.Origin, &pubkeyMsg.Pubkey)

	return nil
}

// ProcessEntryptedMsg is a callback function to handle the received encrypted message
func (m *EncryptionModule) ProcessEntryptedMsg(msg types.Message, pkt transport.Packet) error {
	encryptedMsg, ok := msg.(*types.EncryptedMessage)
	if !ok {
		return xerrors.Errorf("wrong type: %T", msg)
	}

	// decrypt message
	ptxt, err := m.decryptWithPrivkey(*encryptedMsg)
	if err != nil {
		return err
	}

	// process the message locally
	newPkt := transport.Packet{
		Header: pkt.Header,
		Msg:    ptxt,
	}
	err = m.conf.MessageRegistry.ProcessPacket(newPkt)

	return err
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

func (m *EncryptionModule) encryptWithPubkey(msg transport.Message, peer string) (encMsg *types.EncryptedMessage, err error) {
	ptxt, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	pubkey, ok := m.pubkeyStore.get(peer)
	if !ok {
		return nil, xerrors.Errorf("no public key for peer %s", peer)
	}

	hash := sha512.New()
	*encMsg, err = rsa.EncryptOAEP(hash, rand.Reader, (*rsa.PublicKey)(pubkey), ptxt, nil)
	if err != nil {
		return nil, err
	}

	return encMsg, nil
}

// generateKeyPair generates privkey-pubkey pair
func (m *EncryptionModule) decryptWithPrivkey(encMsg types.EncryptedMessage) (msg *transport.Message, err error) {
	hash := sha512.New()
	ptxt, err := rsa.DecryptOAEP(hash, rand.Reader, (*rsa.PrivateKey)(m.privkey), encMsg, nil)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(ptxt, msg)
	if err != nil {
		return nil, err
	}

	return msg, nil
}
