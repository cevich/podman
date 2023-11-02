package e2e_test

import (
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("run basic podman commands", func() {
	var (
		mb      *machineTestBuilder
		testDir string
	)

	BeforeEach(func() {
		testDir, mb = setup()
	})
	AfterEach(func() {
		teardown(originalHomeDir, testDir, mb)
	})

	It("Basic ops", func() {
		// golangci-lint has trouble with actually skipping tests marked Skip
		// so skip it on cirrus envs and where CIRRUS_CI isn't set.
		name := randomString()
		i := new(initMachine)
               session, err := mb.setName(name).setCmd(i.withImagePath(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

               s := new(startMachine)
               start, err := mb.setName(name).setCmd(s).run()
               Expect(err).ToNot(HaveOccurred())
               Expect(start).To(Exit(0))

		bm := basicMachine{}
		imgs, err := mb.setCmd(bm.withPodmanCommand([]string{"images", "-q"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(imgs).To(Exit(0))
		Expect(imgs.outputToStringSlice()).To(BeEmpty())

		newImgs, err := mb.setCmd(bm.withPodmanCommand([]string{"pull", "quay.io/libpod/alpine_nginx"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(newImgs).To(Exit(0))
		Expect(newImgs.outputToStringSlice()).To(HaveLen(1))

		runAlp, err := mb.setCmd(bm.withPodmanCommand([]string{"run", "quay.io/libpod/alpine_nginx", "cat", "/etc/os-release"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(runAlp).To(Exit(0))
		Expect(runAlp.outputToString()).To(ContainSubstring("Alpine Linux"))

		rmCon, err := mb.setCmd(bm.withPodmanCommand([]string{"rm", "-a"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(rmCon).To(Exit(0))
	}, SpecTimeout(time.Minute*15))

	It("Podman ops with port forwarding and gvproxy", func() {
		name := randomString()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withImagePath(mb.imagePath).withNow()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		bm := basicMachine{}
		runAlp, err := mb.setCmd(bm.withPodmanCommand([]string{"run", "-dt", "-p", "62544:80", "quay.io/libpod/alpine_nginx"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(runAlp).To(Exit(0))
		testHTTPServer("62544", false, "podman rulez")

		out, err := pgrep("gvproxy")
		Expect(err).ToNot(HaveOccurred())
		Expect(out).ToNot(BeEmpty())

		rmCon, err := mb.setCmd(bm.withPodmanCommand([]string{"rm", "-af"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(rmCon).To(Exit(0))
		testHTTPServer("62544", true, "")

		stop := new(stopMachine)
		stopSession, err := mb.setCmd(stop).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(stopSession).To(Exit(0))

		// gxproxy should exit after machine is stopped
		out, _ = pgrep("gvproxy")
		Expect(out).ToNot(ContainSubstring("gvproxy"))
	})

})

func testHTTPServer(port string, shouldErr bool, expectedResponse string) {
	address := url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort("localhost", port),
	}

	interval := 250 * time.Millisecond
	var err error
	var resp *http.Response
	for i := 0; i < 6; i++ {
		resp, err = http.Get(address.String())
		if err != nil && shouldErr {
			Expect(err.Error()).To(ContainSubstring(expectedResponse))
			return
		}
		if err == nil {
			defer resp.Body.Close()
			break
		}
		time.Sleep(interval)
		interval *= 2
	}
	Expect(err).ToNot(HaveOccurred())

	body, err := io.ReadAll(resp.Body)
	Expect(err).ToNot(HaveOccurred())
	Expect(string(body)).Should(Equal(expectedResponse))
}
