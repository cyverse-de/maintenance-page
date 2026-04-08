# Project Description

maintenance-page is an application written in the Go programming language that serves up two pages: one is the static maintenance page and the other is a simple admin interface with a toggle that changes the HTTPRoute for the Discovery Environment UI to point to the maintenance page and back again. The intent of this application is to allow users to put the Discovery Environment into and out of maintenance.

It might be useful to look at loading page functionality in the vice-operator service in the app-exposer repository for inspiration. It does a similar swap out for the loading page. For maintenance-page, we want the swap over to occur based on the state of the toggle in the administrator UI rather than on the state of the Discovery Environment.

maintenance-page needs to be able to communicate with the Kubernetes API in order to change the target of the HTTPRoute. It will also need to have two Kubernetes Service definitions loaded in the cluster, one for the maintenance page -- called "maintenance-page" -- and another for the admin page that should be called "maintenance-page-admin". The maintenance-page program should load the K8s Service definitions into the cluster if they're not present, just like the vice-operator does for its own Service definitions.

maintenance-page uses basic auth to protect access to the admin maintenance page. The actual maintenance page does not need authentication.

The design of the maintenance page should be lifted directly from its implementation in Sonora (see ../sonora). The page is currently defined in the public/maintenance.html and public/maintenance.css files. Any additional image files should be located in the Sonora project as well. Please keep the design of the page intact, it should be servable as a static page.

## Configuration

maintenance-page is configurable through command-line flags. Here are the flags:
* --maintenance-page-service - Sets the name of the K8s Service for the loading page. Defaults to "maintenance-page" without the quotes.
* --admin-page-service - Sets the name of the K8s Service for the admin page. Defaults to "maintenance-page-admin" without the quotes.
* --basic-auth-username - Sets the username for the admin page.
* --basic-auth-password - Sets the password for the admin page.

## K8s Definitions

Example files for deploying maintenance-page live in the k8s/ directory.
