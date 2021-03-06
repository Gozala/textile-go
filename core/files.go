package core

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	ipld "gx/ipfs/QmR7TcHkR9nxkUorfi8XMTAMLUK7GiP64TWWBzY3aacc1o/go-ipld-format"
	uio "gx/ipfs/QmfB3oNXGGq9S4B2a9YeCajoATms3Zw2VvDm8fK7VeLSV8/go-unixfs/io"

	"github.com/mr-tron/base58/base58"
	"github.com/textileio/textile-go/crypto"
	"github.com/textileio/textile-go/ipfs"
	m "github.com/textileio/textile-go/mill"
	"github.com/textileio/textile-go/repo"
	"github.com/textileio/textile-go/schema"
)

var ErrFileNotFound = errors.New("file not found")
var ErrMissingFileLink = errors.New("file link not in node")
var ErrMissingDataLink = errors.New("data link not in node")

type Keys map[string]string

type Directory map[string]repo.File

const FileLinkName = "f"
const DataLinkName = "d"

type AddFileConfig struct {
	Input     []byte `json:"input"`
	Use       string `json:"use"`
	Media     string `json:"media"`
	Name      string `json:"name"`
	Plaintext bool   `json:"plaintext"`
}

func (t *Textile) AddFile(mill m.Mill, conf AddFileConfig) (*repo.File, error) {
	var source string
	if conf.Use != "" {
		source = conf.Use
	} else {
		source = t.checksum(conf.Input, conf.Plaintext)
	}

	opts, err := mill.Options(map[string]interface{}{
		"plaintext": conf.Plaintext,
	})
	if err != nil {
		return nil, err
	}

	if efile := t.datastore.Files().GetBySource(mill.ID(), source, opts); efile != nil {
		return efile, nil
	}

	res, err := mill.Mill(conf.Input, conf.Name)
	if err != nil {
		return nil, err
	}

	check := t.checksum(res.File, conf.Plaintext)
	if efile := t.datastore.Files().GetByPrimary(mill.ID(), check); efile != nil {
		return efile, nil
	}

	model := &repo.File{
		Mill:     mill.ID(),
		Checksum: check,
		Source:   source,
		Opts:     opts,
		Media:    conf.Media,
		Name:     conf.Name,
		Size:     len(res.File),
		Added:    time.Now(),
		Meta:     res.Meta,
	}

	var reader *bytes.Reader
	if mill.Encrypt() && !conf.Plaintext {
		key, err := crypto.GenerateAESKey()
		if err != nil {
			return nil, err
		}
		ciphertext, err := crypto.EncryptAES(res.File, key)
		if err != nil {
			return nil, err
		}
		model.Key = base58.FastBase58Encoding(key)
		reader = bytes.NewReader(ciphertext)
	} else {
		reader = bytes.NewReader(res.File)
	}

	hash, err := ipfs.AddData(t.node, reader, mill.Pin())
	if err != nil {
		return nil, err
	}
	model.Hash = hash.Hash().B58String()

	if err := t.datastore.Files().Add(model); err != nil {
		return nil, err
	}

	// return the model fetched from the datastore to ensure
	// consistent date formatting and therefore consistent
	// directory hashes
	return t.datastore.Files().Get(model.Hash), nil
}

func (t *Textile) GetMedia(reader io.Reader, mill m.Mill) (string, error) {
	buffer := make([]byte, 512)
	n, err := reader.Read(buffer)
	if err != nil && err != io.EOF {
		return "", err
	}
	media := http.DetectContentType(buffer[:n])

	return media, mill.AcceptMedia(media)
}

func (t *Textile) AddSchema(jsonstr string, name string) (*repo.File, error) {
	var node schema.Node
	if err := json.Unmarshal([]byte(jsonstr), &node); err != nil {
		return nil, err
	}
	data, err := json.Marshal(&node)
	if err != nil {
		return nil, err
	}

	return t.AddFile(&m.Schema{}, AddFileConfig{
		Input: data,
		Media: "application/json",
		Name:  name,
	})
}

func (t *Textile) AddNodeFromFiles(files []repo.File) (ipld.Node, Keys, error) {
	keys := make(Keys)
	outer := uio.NewDirectory(t.node.DAG)

	for i, file := range files {
		link := strconv.Itoa(i)
		if err := t.fileNode(file, outer, link); err != nil {
			return nil, nil, err
		}
		keys["/"+link+"/"] = file.Key
	}

	node, err := outer.GetNode()
	if err != nil {
		return nil, nil, err
	}
	if err := ipfs.PinNode(t.node, node, false); err != nil {
		return nil, nil, err
	}
	return node, keys, nil
}

func (t *Textile) AddNodeFromDirs(dirs []Directory) (ipld.Node, Keys, error) {
	keys := make(Keys)
	outer := uio.NewDirectory(t.node.DAG)

	for i, dir := range dirs {
		inner := uio.NewDirectory(t.node.DAG)
		olink := strconv.Itoa(i)

		for link, file := range dir {
			if err := t.fileNode(file, inner, link); err != nil {
				return nil, nil, err
			}
			keys["/"+olink+"/"+link+"/"] = file.Key
		}

		node, err := inner.GetNode()
		if err != nil {
			return nil, nil, err
		}
		if err := ipfs.PinNode(t.node, node, false); err != nil {
			return nil, nil, err
		}

		id := node.Cid().Hash().B58String()
		if err := ipfs.AddLinkToDirectory(t.node, outer, olink, id); err != nil {
			return nil, nil, err
		}
	}

	node, err := outer.GetNode()
	if err != nil {
		return nil, nil, err
	}
	if err := ipfs.PinNode(t.node, node, false); err != nil {
		return nil, nil, err
	}
	return node, keys, nil
}

func (t *Textile) File(hash string) (*repo.File, error) {
	file := t.datastore.Files().Get(hash)
	if file == nil {
		return nil, ErrFileNotFound
	}
	return file, nil
}

func (t *Textile) FileData(hash string) (io.ReadSeeker, *repo.File, error) {
	file := t.datastore.Files().Get(hash)
	if file == nil {
		return nil, nil, ErrFileNotFound
	}
	fd, err := ipfs.DataAtPath(t.node, file.Hash)
	if err != nil {
		return nil, nil, err
	}

	var plaintext []byte
	if file.Key != "" {
		key, err := base58.Decode(file.Key)
		if err != nil {
			return nil, nil, err
		}
		plaintext, err = crypto.DecryptAES(fd, key)
		if err != nil {
			return nil, nil, err
		}
	} else {
		plaintext = fd
	}

	return bytes.NewReader(plaintext), file, nil
}

func (t *Textile) TargetNodeKeys(node ipld.Node) (Keys, error) {
	keys := make(Keys)

	for i, link := range node.Links() {
		fn, err := ipfs.NodeAtLink(t.node, link)
		if err != nil {
			return nil, err
		}
		if err := t.fileNodeKeys(fn, i, &keys); err != nil {
			return nil, err
		}
	}

	return keys, nil
}

func (t *Textile) fileNode(file repo.File, dir uio.Directory, link string) error {
	if t.datastore.Files().Get(file.Hash) == nil {
		return ErrFileNotFound
	}

	// remove locally indexed targets
	file.Targets = nil

	plaintext, err := json.Marshal(&file)
	if err != nil {
		return err
	}

	var reader *bytes.Reader
	if file.Key != "" {
		key, err := base58.Decode(file.Key)
		if err != nil {
			return err
		}

		ciphertext, err := crypto.EncryptAES(plaintext, key)
		if err != nil {
			return err
		}

		reader = bytes.NewReader(ciphertext)
	} else {
		reader = bytes.NewReader(plaintext)
	}

	pair := uio.NewDirectory(t.node.DAG)
	if _, err := ipfs.AddDataToDirectory(t.node, pair, FileLinkName, reader); err != nil {
		return err
	}

	if err := ipfs.AddLinkToDirectory(t.node, pair, DataLinkName, file.Hash); err != nil {
		return err
	}

	node, err := pair.GetNode()
	if err != nil {
		return err
	}
	if err := ipfs.PinNode(t.node, node, false); err != nil {
		return err
	}

	return ipfs.AddLinkToDirectory(t.node, dir, link, node.Cid().Hash().B58String())
}

func (t *Textile) fileForPair(pair ipld.Node) (*repo.File, error) {
	d, _, err := pair.ResolveLink([]string{DataLinkName})
	if err != nil {
		return nil, err
	}
	if d == nil {
		return nil, nil
	}
	return t.datastore.Files().Get(d.Cid.Hash().B58String()), nil
}

func (t *Textile) checksum(plaintext []byte, willEncrypt bool) string {
	var add int
	if willEncrypt {
		add = 1
	}
	plaintext = append(plaintext, byte(add))
	sum := sha256.Sum256(plaintext)
	return base58.FastBase58Encoding(sum[:])
}

func (t *Textile) fileNodeKeys(node ipld.Node, index int, keys *Keys) error {
	vkeys := *keys

	if looksLikeFileNode(node) {
		key, err := t.fileLinkKey(node)
		if err != nil {
			return err
		}

		vkeys["/"+strconv.Itoa(index)+"/"] = key
	} else {
		for _, link := range node.Links() {
			n, err := ipfs.NodeAtLink(t.node, link)
			if err != nil {
				return err
			}

			key, err := t.fileLinkKey(n)
			if err != nil {
				return err
			}

			vkeys["/"+strconv.Itoa(index)+"/"+link.Name+"/"] = key
		}
	}
	keys = &vkeys

	return nil
}

func (t *Textile) fileLinkKey(inode ipld.Node) (string, error) {
	dlink := schema.LinkByName(inode.Links(), DataLinkName)
	if dlink == nil {
		return "", ErrMissingDataLink
	}

	file := t.datastore.Files().Get(dlink.Cid.Hash().B58String())
	if file == nil {
		return "", ErrFileNotFound
	}
	return file.Key, nil
}

// looksLikeFileNode returns whether or not a node appears to
// be a textile node. It doesn't inspect the actual data.
func looksLikeFileNode(node ipld.Node) bool {
	links := node.Links()
	if len(links) != 2 {
		return false
	}
	if schema.LinkByName(links, FileLinkName) == nil ||
		schema.LinkByName(links, DataLinkName) == nil {
		return false
	}
	return true
}
