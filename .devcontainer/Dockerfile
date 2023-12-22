FROM --platform=linux/amd64 ubuntu:jammy

LABEL description="API Client development container"

# Install base dependencies
RUN apt-get update && apt-get -y upgrade && \
  apt-get install -y ca-certificates curl gnupg lsb-release software-properties-common \
  g++ clang lcov clang-format clang-tidy clangd \
  openssh-server unzip groff man \ 
  emacs vim zsh jq

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

# Install Trunk
RUN curl https://get.trunk.io -fsSL | bash -s -- -y

# Unminimize to get docs
RUN yes | unminimize

# expose port 22 
EXPOSE 22