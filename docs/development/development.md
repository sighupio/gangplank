# Development FAQ

### **How did we fork the Gangplank project?**

<details>
  <summary>Answer</summary>
  
We forked the repository [https://github.com/vmware-archive/gangway](https://github.com/vmware-archive/gangway) because it was no longer active and maintained. This allowed us to continue developing the project under our own management, as the original repository had been archived and was not receiving updates or fixes.

</details>

---

### **What did we change from the upstream project?**

<details>
  <summary>Answer</summary>
  
We updated all the project dependencies to ensure compatibility with newer versions of libraries and tools. The Go version in the `go.mod` file was also updated to a more recent one. In addition, we renamed all the project packages to point to our fork [https://github.com/sighup/gangplank](https://github.com/sighup/gangplank). Lastly, we adapted the release process to match our standards for versioning, packaging, and deployment. We aligned the UI with the fury-ui style.

</details>

---

### **Which features did we add?**

<details>
  <summary>Answer</summary>
  
We added the `GANGPLANK_CONFIG_REMOVE_CA_FROM_KUBECONFIG` configuration to remove the CA from the kubeconfig and the `GANGPLANK_CONFIG_NAMESPACE` configuration to set a default namespace.

</details>

---

### **How do I set up a local development environment and how do I test the software?**

<details>
  <summary>Answer</summary>
  
To set up a local development environment, you can use the script `make dev-up`. This will create a local Kubernetes cluster using Kind, and it will install both Dex and Gangplank directly from the local source code. Once you are done with development or testing, you can tear down the environment by running `make dev-down`. This will clean up the Kubernetes cluster and any resources created during the session.

</details>
