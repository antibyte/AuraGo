# Jupyter Docker Stacks 2026-10-26, OCI index includes linux/amd64 and linux/arm64.
FROM quay.io/jupyter/scipy-notebook@sha256:0698ec1bc3ecf0ff30e571af74816459f1a2947b560236af0d28e13900a3b296
USER root
COPY bridge.py /opt/aurago/bridge.py
RUN chmod 0555 /opt/aurago/bridge.py && mkdir -p /workspace && chown 1000:100 /workspace
USER 1000:100
WORKDIR /workspace
ENV HOME=/tmp/home JUPYTER_RUNTIME_DIR=/tmp/runtime IPYTHONDIR=/tmp/ipython
LABEL org.aurago.runtime="kaggle-notebook" org.aurago.protocol="1"
ENTRYPOINT ["/usr/bin/tini", "--"]
CMD ["sleep", "infinity"]
