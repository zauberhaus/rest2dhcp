package main_test

import (
	"context"
	"fmt"
	"net"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/zauberhaus/rest2dhcp/client"
)

var _ = Describe("Rest2dhcp", func() {
	Context("Integration test", func() {
		server := "localhost"
		port := 8080

		ipnet := net.IPNet{
			IP:   net.IP{172, 17, 0, 1},
			Mask: net.IPMask{255, 255, 0, 0},
		}

		When("rest2dhcp is running", func() {

			c := client.NewClient(fmt.Sprintf("http://%v:%v", server, port))

			ctx := context.Background()

			hostname := "test12345"

			When("Lease is requested", func() {
				response, err := c.Lease(ctx, hostname, nil)

				It("error is nil", func() {
					Expect(err).To(BeNil())
				})

				It("response is ok", func() {
					Expect(response).To(Not(BeNil()))
					Expect(ipnet.Contains(response.IP)).To(BeTrue())
					Expect(response.Mask.To4()).To(Equal(net.IP(ipnet.Mask)))
					Expect(ipnet.Contains(response.Router)).To(BeTrue())
					Expect(response.DNS.To4()).To(Not(BeNil()))
				})

				if response != nil {
					When("Lease is renewal is requested", func() {
						response2, err := c.Renew(ctx, response.Hostname, response.Mac, response.IP)

						It("error is nil", func() {
							Expect(err).To(BeNil())
						})

						It("response is ok", func() {
							Expect(response2).To(Not(BeNil()))
							Expect(response2.IP).To(BeEquivalentTo(response.IP))
							Expect(response2.Mask).To(BeEquivalentTo(response.Mask))
							Expect(response2.Router).To(BeEquivalentTo(response.Router))
							Expect(response2.DNS).To(BeEquivalentTo(response.DNS))
							Expect(response2.Renew.After(response.Renew)).To(BeTrue())
							Expect(response2.Rebind.After(response.Rebind)).To(BeTrue())
							Expect(response2.Expire.After(response.Expire)).To(BeTrue())
						})

						if response2 != nil {
							When("Renewal of unknwon mac is requested", func() {
								_, err := c.Renew(ctx, response.Hostname, client.MAC{1, 2, 3, 4, 5, 6}, response.IP)

								It("Returns NAK", func() {
									Expect(err).To(Not(BeNil()))
									clerr, ok := err.(*client.Error)
									if Expect(ok).To(BeTrue()) {
										Expect(clerr.Code()).To(Equal(406))
									}
								})
							})

							When("Renewal of unknwon ip is requested", func() {
								_, err := c.Renew(ctx, response.Hostname, response.Mac, net.IP{1, 2, 3, 4})

								It("Returns NAK", func() {
									Expect(err).To(Not(BeNil()))
									clerr, ok := err.(*client.Error)
									if Expect(ok).To(BeTrue()) {
										Expect(clerr.Code()).To(Equal(406))
									}
								})
							})

							When("Lease is released", func() {
								err := c.Release(ctx, response.Hostname, response.Mac, response.IP)

								It("error is nil", func() {
									Expect(err).To(BeNil())
								})
							})
						}
					})
				}
			})
		})
	})
})
