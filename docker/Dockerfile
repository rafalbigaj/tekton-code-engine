FROM registry.access.redhat.com/ubi8/ubi

USER root

RUN mkdir -p /home/ibmcloud && \
    chown -R 1001:0 /home/ibmcloud && \
    chmod 777 /home/ibmcloud

ENV HOME /home/ibmcloud

RUN curl -fsSL https://clis.cloud.ibm.com/install/linux | sh

USER 1000660001

RUN ibmcloud plugin install code-engine

ENTRYPOINT ["ibmcloud", "--help"]