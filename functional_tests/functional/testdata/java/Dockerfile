FROM tomcat:11.0-jre21

COPY start.sh .

RUN rm -Rf webapps && mv webapps.dist webapps

ENTRYPOINT ["bash", "start.sh"]
