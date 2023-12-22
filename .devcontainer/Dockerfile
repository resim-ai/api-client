FROM --platform=linux/amd64 ubuntu:jammy

LABEL description="API Client development container"

# Install base dependencies
RUN apt-get update && apt-get -y upgrade && \
  apt-get install -y ca-certificates curl gnupg lsb-release software-properties-common \
  g++ clang lcov clang-format clang-tidy clangd \
  openssh-server unzip groff man \ 
  emacs vim zsh jq

# Install Docker
RUN curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg
RUN echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu \
  $(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null
RUN apt-get update
RUN apt-get install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin

# Install Github CLI
RUN curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg
RUN echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" | tee /etc/apt/sources.list.d/github-cli.list > /dev/null
RUN apt-get update
RUN apt-get install -y gh

# configure SSH for communication with Visual Studio 
RUN mkdir -p /var/run/sshd
RUN echo 'PasswordAuthentication yes' >> /etc/ssh/sshd_config && \ 
  ssh-keygen -A 

# Install bash completion
RUN apt-get install -y bash-completion
RUN echo "source /etc/bash_completion" >> /root/.bashrc

# Install AWS CLI v2.6.1
# TODO: Verify signature of zip file.  This requires having a local gpg key and signing
# the AWS CLI key.
RUN wget -O awscliv2.zip https://awscli.amazonaws.com/awscli-exe-linux-x86_64-2.6.1.zip
RUN unzip awscliv2.zip && aws/install && rm -rf aws awscliv2.zip

# Install Amazon ECR Credential Helper
RUN apt install -y amazon-ecr-credential-helper

# Install Go
RUN wget https://go.dev/dl/go1.21.3.linux-amd64.tar.gz
RUN rm -rf /usr/local/go && tar -C /usr/local -xzf go1.21.3.linux-amd64.tar.gz
ENV PATH "${PATH}:/usr/local/go/bin:/root/go/bin"

# Install go tools
RUN go install -v golang.org/x/tools/gopls@v0.10.1
RUN go install -v github.com/go-delve/delve/cmd/dlv@v1.9.1
RUN go install -v github.com/haya14busa/goplay/cmd/goplay@v1.0.0
RUN go install -v github.com/fatih/gomodifytags@v1.16.0
RUN go install -v github.com/josharian/impl@v1.1.0
RUN go install -v github.com/cweill/gotests/gotests@v1.6.0
RUN go install -v honnef.co/go/tools/cmd/staticcheck@v0.3.3

# Install golang-migrate
RUN go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Install Terraform
RUN curl -fsSL https://apt.releases.hashicorp.com/gpg | apt-key add -
RUN apt-add-repository "deb [arch=amd64] https://apt.releases.hashicorp.com $(lsb_release -cs) main"
RUN apt-get update && apt-get -y install terraform

# Install Trunk
RUN curl https://get.trunk.io -fsSL | bash -s -- -y

# Install the AWS Session Manager plugin
RUN curl "https://s3.amazonaws.com/session-manager-downloads/plugin/latest/ubuntu_64bit/session-manager-plugin.deb" -o "session-manager-plugin.deb" && \
  dpkg -i session-manager-plugin.deb

# Unminimize to get docs
RUN yes | unminimize

# expose port 22 
EXPOSE 22