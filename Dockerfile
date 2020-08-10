FROM alpine

RUN apk update && apk upgrade && apk add curl

ENV RELAY 192.168.1.9
ENV SERVER 192.168.1.1
ENV MODE fritzbox

COPY rest2dhcp /
COPY run.sh /

CMD /run.sh $RELAY $SERVER $MODE

