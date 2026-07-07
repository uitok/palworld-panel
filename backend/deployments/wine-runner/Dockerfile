FROM scottyhardy/docker-wine:latest@sha256:477aae36af41923cfb5eefb23923b035f8010caa49eaded952316f937dd8a49b

ENV DEBIAN_FRONTEND=noninteractive \
    WINEDEBUG=-all \
    WINEPREFIX=/data/wineprefix \
    HOME=/root

ADD https://steamcdn-a.akamaihd.net/client/installer/steamcmd_linux.tar.gz /tmp/steamcmd_linux.tar.gz
RUN mkdir -p /opt/steamcmd \
    && tar -xzf /tmp/steamcmd_linux.tar.gz -C /opt/steamcmd \
    && rm -f /tmp/steamcmd_linux.tar.gz

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
